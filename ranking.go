package main

import (
	"errors"
	"net/url"
	"regexp"
)

// Genre is one ranking (or category) TVer offers, e.g. "drama" / "ドラマ".
type Genre struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

var genreIDRe = regexp.MustCompile(`^[a-z0-9_-]{1,40}$`)

// rankingAPI maps a ranking kind to its list and detail endpoints. TVer keeps
// separate rankings of programs ("series") and of episodes ("episode").
func rankingAPI(kind string) (list, detail string, err error) {
	switch kind {
	case "series":
		return "callRanking", "callRankingDetail", nil
	case "episode":
		return "callEpisodeRanking", "callEpisodeRankingDetail", nil
	}
	return "", "", errors.New("unknown ranking kind: " + kind)
}

// RankingGenres lists the genres a ranking is split into ("総合", "ドラマ", ...).
// kind is "series" or "episode".
func (t *Tver) RankingGenres(kind string) ([]Genre, error) {
	list, _, err := rankingAPI(kind)
	if err != nil {
		return nil, err
	}
	var r struct {
		Result struct {
			Contents []struct {
				Content struct {
					ID    string `json:"id"`
					Title string `json:"title"`
				} `json:"content"`
			} `json:"contents"`
		} `json:"result"`
	}
	if err := t.get(serviceAPI+"/api/v1/"+list, nil, &r); err != nil {
		return nil, err
	}
	genres := make([]Genre, 0, len(r.Result.Contents))
	for _, c := range r.Result.Contents {
		if c.Content.ID != "" {
			genres = append(genres, Genre{ID: c.Content.ID, Title: c.Content.Title})
		}
	}
	return genres, nil
}

// Ranking returns the ranking for one genre, up to 50 entries with Item.Rank set.
// kind is "series" or "episode"; genreID is a Genre.ID such as "all" or "drama".
func (t *Tver) Ranking(kind, genreID string) ([]Item, error) {
	_, detail, err := rankingAPI(kind)
	if err != nil {
		return nil, err
	}
	if !genreIDRe.MatchString(genreID) {
		return nil, errors.New("invalid genre: " + genreID)
	}
	var r struct {
		Result struct {
			Contents struct {
				Contents []rawNode `json:"contents"`
			} `json:"contents"`
		} `json:"result"`
	}
	if err := t.get(serviceAPI+"/api/v1/"+detail+"/"+url.PathEscape(genreID), nil, &r); err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(r.Result.Contents.Contents))
	for _, n := range r.Result.Contents.Contents {
		if it, ok := nodeToItem(n); ok && (it.Type == "episode" || it.Type == "series") {
			items = append(items, it)
		}
	}
	return items, nil
}
