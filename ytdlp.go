package main

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"time"
)

// streamYtdlp asks yt-dlp (from PATH) for the episode's master HLS manifest.
// TVer serves video and audio as separate renditions, so the master manifest
// (which hls.js plays as is) is used instead of a single format URL.
func streamYtdlp(episodeID string) (Stream, error) {
	bin, _ := ytdlpBinary()
	if bin == "" {
		return Stream{}, errors.New("yt-dlp が見つかりません(設定画面からダウンロードできます)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "--no-playlist", "--no-warnings", "-J",
		"https://tver.jp/episodes/"+episodeID)
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && len(ee.Stderr) > 0 {
			return Stream{}, errors.New(strings.TrimSpace(string(ee.Stderr)))
		}
		return Stream{}, err
	}
	var info struct {
		Title       string `json:"title"`
		URL         string `json:"url"`
		ManifestURL string `json:"manifest_url"`
		Formats     []struct {
			URL         string `json:"url"`
			ManifestURL string `json:"manifest_url"`
		} `json:"formats"`
	}
	if err := json.Unmarshal(out, &info); err != nil {
		return Stream{}, errors.New("yt-dlp の出力を解釈できません: " + err.Error())
	}
	u := info.ManifestURL
	for i := len(info.Formats) - 1; u == "" && i >= 0; i-- {
		u = info.Formats[i].ManifestURL
	}
	if u == "" {
		u = info.URL
	}
	if !strings.HasPrefix(u, "http") {
		return Stream{}, errors.New("yt-dlp が再生 URL を返しませんでした")
	}
	return Stream{Title: info.Title, URL: u}, nil
}
