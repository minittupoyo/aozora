package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	platformAPI = "https://platform-api.tver.jp"
	staticsAPI  = "https://statics.tver.jp"
	contentsAPI = "https://contents-api.tver.jp"
	serviceAPI  = "https://service-api.tver.jp"
	userAgent   = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36"
)

// Tver talks to TVer's unofficial web API. It runs in Go so the page is not
// bound by CORS.
type Tver struct {
	mu     sync.Mutex
	uid    string
	token  string
	client http.Client
	pkeys  map[string]string // Brightcove policy keys by account/player
}

// Item is an episode, series or live entry shown as a card.
type Item struct {
	ID        string `json:"id"`
	Type      string `json:"type"` // episode, series, live, ...
	Title     string `json:"title"`
	Subtitle  string `json:"subtitle"`
	Thumbnail string `json:"thumbnail"`
	// Rank is the position in a ranking (1-based), or 0 outside rankings.
	Rank int `json:"rank"`
}

// Section is a titled row of items on the home screen.
type Section struct {
	Title string `json:"title"`
	Items []Item `json:"items"`
	// Category is "drama", "variety", "anime", "news", "sports" or "others" for
	// sections that belong to a genre, and empty for the rest (new, ending soon, ...).
	Category string `json:"category"`
}

// categoryOf derives a section's genre from TVer's component ID, e.g.
// "drama.catchup.popular" or "ranking-anime.".
func categoryOf(componentID string) string {
	ranking := strings.HasPrefix(componentID, "ranking-")
	key := strings.SplitN(strings.TrimPrefix(componentID, "ranking-"), ".", 2)[0]
	switch key {
	case "drama", "variety", "anime", "news", "sports", "others":
		return key
	case "kansai", "local":
		if ranking {
			return "variety" // regional variety rankings
		}
		return "others"
	}
	return ""
}

// Stream is a playable HLS source.
type Stream struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

func (t *Tver) get(u string, header map[string]string, out any) error {
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("x-tver-platform-type", "web")
	for k, v := range header {
		req.Header.Set(k, v)
	}
	return t.do(req, out)
}

func (t *Tver) do(req *http.Request, out any) error {
	t.client.Timeout = 20 * time.Second
	res, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode >= 400 {
		return fmt.Errorf("%s: HTTP %d", req.URL.Host, res.StatusCode)
	}
	if s, ok := out.(*string); ok {
		*s = string(body)
		return nil
	}
	return json.Unmarshal(body, out)
}

// session lazily creates the anonymous platform user TVer requires.
func (t *Tver) session() (uid, token string, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.uid != "" {
		return t.uid, t.token, nil
	}
	req, err := http.NewRequest("POST", platformAPI+"/v2/api/platform_users/browser/create",
		strings.NewReader("device_type=pc"))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", userAgent)
	var r struct {
		Result struct {
			UID   string `json:"platform_uid"`
			Token string `json:"platform_token"`
		} `json:"result"`
	}
	if err := t.do(req, &r); err != nil {
		return "", "", err
	}
	if r.Result.UID == "" {
		return "", "", errors.New("TVer session could not be created")
	}
	t.uid, t.token = r.Result.UID, r.Result.Token
	return t.uid, t.token, nil
}

func (t *Tver) platformURL(path string, q url.Values) (string, error) {
	uid, token, err := t.session()
	if err != nil {
		return "", err
	}
	q.Set("platform_uid", uid)
	q.Set("platform_token", token)
	return platformAPI + path + "?" + q.Encode(), nil
}

// rawNode is the loosely typed shape of TVer's content nodes.
type rawNode struct {
	Type     string          `json:"type"`
	Content  json.RawMessage `json:"content"`
	Contents []rawNode       `json:"contents"`
	Title    string          `json:"title"`
	Label    string          `json:"label"`

	ComponentID string `json:"componentID"`
	Rank        int    `json:"rank"`
}

type rawContent struct {
	ID                 string `json:"id"`
	Title              string `json:"title"`
	SeriesTitle        string `json:"seriesTitle"`
	BroadcastDateLabel string `json:"broadcastDateLabel"`
	BroadcasterName    string `json:"broadcasterName"`
	Version            int    `json:"version"`
	ThumbnailPath      string `json:"thumbnailPath"`
	SeriesID           string `json:"seriesID"`
}

func thumb(typ, id string) string {
	switch typ {
	case "series":
		return "https://statics.tver.jp/images/content/thumbnail/series/small/" + id + ".jpg"
	case "live":
		return ""
	}
	return "https://statics.tver.jp/images/content/thumbnail/episode/small/" + id + ".jpg"
}

func nodeToItem(n rawNode) (Item, bool) {
	if len(n.Content) == 0 {
		return Item{}, false
	}
	var c rawContent
	if json.Unmarshal(n.Content, &c) != nil || c.ID == "" {
		return Item{}, false
	}
	it := Item{ID: c.ID, Type: n.Type, Title: c.Title, Thumbnail: thumb(n.Type, c.ID), Rank: n.Rank}
	if c.ThumbnailPath != "" {
		it.Thumbnail = staticsAPI + "/" + strings.TrimPrefix(c.ThumbnailPath, "/")
	}
	if n.Type == "episode" {
		it.Title, it.Subtitle = c.SeriesTitle, c.Title
		if it.Title == "" {
			it.Title, it.Subtitle = c.Title, ""
		}
		it.Subtitle = strings.TrimSpace(c.BroadcasterName + " " + c.BroadcastDateLabel)
		if c.SeriesTitle != "" {
			it.Subtitle = c.Title + "\n" + it.Subtitle
		}
	}
	return it, true
}

func collect(nodes []rawNode, out *[]Section) {
	for _, n := range nodes {
		if len(n.Contents) > 0 {
			sec := Section{Title: n.Label, Category: categoryOf(n.ComponentID)}
			if sec.Title == "" {
				sec.Title = n.Title
			}
			if sec.Title == "BILLBOARD" {
				sec.Title = "注目"
			}
			for _, c := range n.Contents {
				if it, ok := nodeToItem(c); ok && (it.Type == "episode" || it.Type == "series") {
					sec.Items = append(sec.Items, it)
				}
			}
			if len(sec.Items) > 0 {
				if sec.Title == "" {
					sec.Title = "おすすめ"
				}
				*out = append(*out, sec)
			}
			collect(n.Contents, out)
		}
	}
}

// Home returns the sections of TVer's top page.
func (t *Tver) Home() ([]Section, error) {
	// The web client reads the home page from the contents API, which needs no session.
	var raw string
	if err := t.get(contentsAPI+"/contents/api/v1/callHome", nil, &raw); err != nil {
		return nil, err
	}
	var r struct {
		Components []rawNode `json:"components"`
	}
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		return nil, err
	}
	var secs []Section
	collect(r.Components, &secs)
	if len(secs) == 0 {
		return nil, fmt.Errorf("ホームのデータを解釈できませんでした: %s", describeHome(raw))
	}
	return secs, nil
}

// describeHome summarizes the shape of a callHome response for error reports.
func describeHome(raw string) string {
	var top map[string]json.RawMessage
	if json.Unmarshal([]byte(raw), &top) != nil {
		return "JSONではありません: " + truncate(raw, 200)
	}
	var res map[string]json.RawMessage
	json.Unmarshal(top["result"], &res)
	keys := func(m map[string]json.RawMessage) string {
		var ks []string
		for k := range m {
			ks = append(ks, k)
		}
		return strings.Join(ks, ",")
	}
	desc := fmt.Sprintf("top=[%s] result=[%s] code=%s type=%s message=%s", keys(top), keys(res),
		truncate(string(top["code"]), 40), truncate(string(top["type"]), 40), truncate(string(top["message"]), 200))
	var comps []map[string]json.RawMessage
	if json.Unmarshal(res["components"], &comps) == nil && len(comps) > 0 {
		desc += fmt.Sprintf(" components=%d first=[%s] first.type=%s", len(comps), keys(comps[0]), truncate(string(comps[0]["type"]), 40))
		var kids []map[string]json.RawMessage
		if json.Unmarshal(comps[0]["contents"], &kids) == nil && len(kids) > 0 {
			desc += fmt.Sprintf(" first.contents[0]=[%s]", keys(kids[0]))
		}
	}
	return desc
}

func truncate(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

// Search finds programs and episodes by keyword. TVer's keyword search returns
// episodes only, so the programs (series) are the ones those episodes belong to;
// those whose title contains the keyword come first.
func (t *Tver) Search(keyword string) ([]Section, error) {
	var r struct {
		Result struct {
			Contents []rawNode `json:"contents"`
		} `json:"result"`
	}
	if err := t.get(serviceAPI+"/api/v1/callKeywordSearch?"+url.Values{"keyword": {keyword}}.Encode(), nil, &r); err != nil {
		return nil, err
	}

	var episodes []Item
	var series []Item
	index := map[string]int{} // seriesID -> position in series
	hits := map[string]int{}  // seriesID -> matching episodes
	for _, n := range r.Result.Contents {
		it, ok := nodeToItem(n)
		if !ok || it.Type != "episode" {
			continue
		}
		episodes = append(episodes, it)
		var c rawContent
		json.Unmarshal(n.Content, &c)
		if c.SeriesID == "" {
			continue
		}
		hits[c.SeriesID]++
		if _, seen := index[c.SeriesID]; !seen && c.SeriesTitle != "" {
			index[c.SeriesID] = len(series)
			series = append(series, Item{
				ID: c.SeriesID, Type: "series", Title: c.SeriesTitle,
				Thumbnail: staticsAPI + "/images/content/thumbnail/series/xlarge/" + c.SeriesID + ".jpg",
			})
		}
	}
	for i := range series {
		series[i].Subtitle = fmt.Sprintf("%d件のエピソードがヒット", hits[series[i].ID])
	}
	kw := strings.ToLower(strings.TrimSpace(keyword))
	sort.SliceStable(series, func(i, j int) bool {
		a := strings.Contains(strings.ToLower(series[i].Title), kw)
		b := strings.Contains(strings.ToLower(series[j].Title), kw)
		return a && !b
	})

	var secs []Section
	if len(series) > 0 {
		secs = append(secs, Section{Title: "番組", Items: series})
	}
	if len(episodes) > 0 {
		secs = append(secs, Section{Title: "エピソード", Items: episodes})
	}
	return secs, nil
}

// SeriesEpisodes lists the episodes of a series, one section per season
// ("本編", "メイキング", "ダイジェスト", ...), in the order TVer gives them.
func (t *Tver) SeriesEpisodes(seriesID string) ([]Section, error) {
	var seasons struct {
		Result struct {
			Contents []struct {
				Content struct {
					ID    string `json:"id"`
					Title string `json:"title"`
				} `json:"content"`
			} `json:"contents"`
		} `json:"result"`
	}
	if err := t.get(serviceAPI+"/api/v1/callSeriesSeasons/"+url.PathEscape(seriesID), nil, &seasons); err != nil {
		return nil, err
	}
	var secs []Section
	for _, s := range seasons.Result.Contents {
		var eps struct {
			Result struct {
				Contents []rawNode `json:"contents"`
			} `json:"result"`
		}
		if err := t.get(serviceAPI+"/api/v1/callSeasonEpisodes/"+url.PathEscape(s.Content.ID), nil, &eps); err != nil {
			return nil, err
		}
		sec := Section{Title: s.Content.Title}
		if sec.Title == "" {
			sec.Title = "エピソード"
		}
		for _, n := range eps.Result.Contents {
			if it, ok := nodeToItem(n); ok {
				sec.Items = append(sec.Items, it)
			}
		}
		if len(sec.Items) > 0 {
			secs = append(secs, sec)
		}
	}
	return secs, nil
}

var policyKeyRe = regexp.MustCompile(`policyKey\s*:\s*"([^"]+)"`)

func (t *Tver) policyKey(account, player string) (string, error) {
	t.mu.Lock()
	if t.pkeys == nil {
		t.pkeys = map[string]string{}
	}
	k := account + "/" + player
	if v, ok := t.pkeys[k]; ok {
		t.mu.Unlock()
		return v, nil
	}
	t.mu.Unlock()

	var js string
	if err := t.get(fmt.Sprintf("https://players.brightcove.net/%s/%s_default/index.min.js", account, player), nil, &js); err != nil {
		return "", err
	}
	m := policyKeyRe.FindStringSubmatch(js)
	if m == nil {
		return "", errors.New("Brightcove policy key not found")
	}
	t.mu.Lock()
	t.pkeys[k] = m[1]
	t.mu.Unlock()
	return m[1], nil
}

// The page resolves a stream in steps so it can show its progress:
// StreamDirect, then StreamYtdlp if that fails, then Proxy on the result.

// StreamDirect resolves an episode to an HLS URL through Brightcove.
func (t *Tver) StreamDirect(episodeID string) (Stream, error) {
	return t.streamNative(episodeID)
}

// StreamYtdlp resolves an episode to an HLS URL with yt-dlp from PATH.
func (t *Tver) StreamYtdlp(episodeID string) (Stream, error) {
	return streamYtdlp(episodeID)
}

// Proxy returns a loopback URL that serves streamURL with TVer's Origin.
func (t *Tver) Proxy(streamURL string) (string, error) {
	return proxied(streamURL)
}

func (t *Tver) streamNative(episodeID string) (Stream, error) {
	var ep struct {
		Title string `json:"title"`
		Video struct {
			AccountID string `json:"accountID"`
			PlayerID  string `json:"playerID"`
			VideoRef  string `json:"videoRefID"`
			VideoID   string `json:"videoID"`
		} `json:"video"`
	}
	if err := t.get(fmt.Sprintf("%s/content/episode/%s.json?v=%d", staticsAPI, url.PathEscape(episodeID), time.Now().Unix()/60), nil, &ep); err != nil {
		return Stream{}, err
	}
	v := ep.Video
	if v.AccountID == "" || (v.VideoRef == "" && v.VideoID == "") {
		return Stream{}, errors.New("このエピソードは再生情報がありません")
	}
	ref := "ref%3A" + url.PathEscape(v.VideoRef)
	if v.VideoRef == "" {
		ref = v.VideoID
	}
	pk, err := t.policyKey(v.AccountID, v.PlayerID)
	if err != nil {
		return Stream{}, err
	}
	var bc struct {
		Sources []struct {
			Src        string         `json:"src"`
			Type       string         `json:"type"`
			KeySystems map[string]any `json:"key_systems"`
		} `json:"sources"`
	}
	api := fmt.Sprintf("https://edge.api.brightcove.com/playback/v1/accounts/%s/videos/%s", v.AccountID, ref)
	if err := t.get(api, map[string]string{"Accept": "application/json;pk=" + pk}, &bc); err != nil {
		return Stream{}, err
	}
	for _, s := range bc.Sources {
		if strings.HasPrefix(s.Src, "https://") && len(s.KeySystems) == 0 &&
			(strings.Contains(s.Type, "mpegURL") || strings.Contains(s.Src, ".m3u8")) {
			return Stream{Title: ep.Title, URL: s.Src}, nil
		}
	}
	return Stream{}, errors.New("再生可能なHLSストリームが見つかりません（DRM保護または地域制限の可能性）")
}
