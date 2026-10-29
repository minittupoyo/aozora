package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const ytdlpReleases = "https://github.com/yt-dlp/yt-dlp/releases/latest/download/"

var ytdlpInstallMu sync.Mutex

// YtdlpInfo describes the yt-dlp the app would use.
type YtdlpInfo struct {
	// Source is "managed" (the app's own copy), "path" (found on PATH) or "none".
	Source  string `json:"source"`
	Path    string `json:"path"`
	Version string `json:"version"`
	// Managed reports whether the app's own copy exists, whichever is in use.
	Managed     bool   `json:"managed"`
	ManagedPath string `json:"managedPath"`
}

func ytdlpAsset() (string, error) {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "windows/amd64":
		return "yt-dlp.exe", nil
	case "windows/arm64":
		return "yt-dlp_arm64.exe", nil
	case "windows/386":
		return "yt-dlp_x86.exe", nil
	case "linux/amd64":
		return "yt-dlp_linux", nil
	case "linux/arm64":
		return "yt-dlp_linux_aarch64", nil
	case "darwin/amd64", "darwin/arm64":
		return "yt-dlp_macos", nil
	}
	return "", fmt.Errorf("この環境(%s/%s)用の yt-dlp は自動取得に対応していません", runtime.GOOS, runtime.GOARCH)
}

// managedYtdlpPath is where the app keeps its own copy of yt-dlp.
func managedYtdlpPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	name := "yt-dlp"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(dir, "Aozora", "bin", name), nil
}

// ytdlpBinary picks the app's own copy over the one on PATH.
func ytdlpBinary() (path, source string) {
	if p, err := managedYtdlpPath(); err == nil {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, "managed"
		}
	}
	if p, err := exec.LookPath("yt-dlp"); err == nil {
		return p, "path"
	}
	return "", "none"
}

func ytdlpVersion(bin string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "--version")
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func ytdlpInfo() YtdlpInfo {
	info := YtdlpInfo{Source: "none"}
	if mp, err := managedYtdlpPath(); err == nil {
		info.ManagedPath = mp
		if st, err := os.Stat(mp); err == nil && !st.IsDir() {
			info.Managed = true
		}
	}
	if bin, src := ytdlpBinary(); bin != "" {
		info.Path, info.Source = bin, src
		info.Version = ytdlpVersion(bin)
	}
	return info
}

// YtdlpStatus reports which yt-dlp the app uses and its version.
func (t *Tver) YtdlpStatus() YtdlpInfo {
	return ytdlpInfo()
}

// YtdlpInstall downloads the latest official yt-dlp release into the app's own
// folder, verifying it against the release's SHA2-256SUMS. Running it again updates.
func (t *Tver) YtdlpInstall() (YtdlpInfo, error) {
	ytdlpInstallMu.Lock()
	defer ytdlpInstallMu.Unlock()

	asset, err := ytdlpAsset()
	if err != nil {
		return YtdlpInfo{}, err
	}
	dest, err := managedYtdlpPath()
	if err != nil {
		return YtdlpInfo{}, err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return YtdlpInfo{}, err
	}

	client := &http.Client{Timeout: 10 * time.Minute}
	want, err := fetchChecksum(client, asset)
	if err != nil {
		return YtdlpInfo{}, fmt.Errorf("チェックサムを取得できませんでした: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(dest), "yt-dlp-*.part")
	if err != nil {
		return YtdlpInfo{}, err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after a successful rename

	got, err := download(client, ytdlpReleases+asset, tmp)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return YtdlpInfo{}, fmt.Errorf("ダウンロードに失敗しました: %w", err)
	}
	if got != want {
		return YtdlpInfo{}, errors.New("ダウンロードしたファイルのチェックサムが一致しません")
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		return YtdlpInfo{}, err
	}
	if ytdlpVersion(tmpName) == "" {
		return YtdlpInfo{}, errors.New("ダウンロードした yt-dlp を実行できませんでした")
	}
	if err := os.Rename(tmpName, dest); err != nil {
		return YtdlpInfo{}, fmt.Errorf("保存に失敗しました: %w", err)
	}
	return ytdlpInfo(), nil
}

// YtdlpRemove deletes the app's own copy; PATH is used again if it has one.
func (t *Tver) YtdlpRemove() (YtdlpInfo, error) {
	ytdlpInstallMu.Lock()
	defer ytdlpInstallMu.Unlock()
	dest, err := managedYtdlpPath()
	if err != nil {
		return YtdlpInfo{}, err
	}
	if err := os.Remove(dest); err != nil && !errors.Is(err, os.ErrNotExist) {
		return YtdlpInfo{}, err
	}
	return ytdlpInfo(), nil
}

func httpGet(client *http.Client, url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		res.Body.Close()
		return nil, fmt.Errorf("HTTP %d", res.StatusCode)
	}
	return res, nil
}

// fetchChecksum reads the asset's SHA-256 from the release's SHA2-256SUMS.
func fetchChecksum(client *http.Client, asset string) (string, error) {
	res, err := httpGet(client, ytdlpReleases+"SHA2-256SUMS")
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	sc := bufio.NewScanner(io.LimitReader(res.Body, 1<<20))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) == 2 && strings.TrimPrefix(f[1], "*") == asset {
			return strings.ToLower(f[0]), nil
		}
	}
	if err := sc.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("%s のチェックサムが見つかりません", asset)
}

// download copies url into w and returns the SHA-256 of what was written.
func download(client *http.Client, url string, w io.Writer) (string, error) {
	res, err := httpGet(client, url)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(w, h), io.LimitReader(res.Body, 300<<20)); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
