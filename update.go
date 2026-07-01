package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const markoReleaseLatestURL = "https://api.github.com/repos/anmacmillan/marko/releases/latest"

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func updateMarko() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate current binary: %w", err)
	}
	return updateMarkoFrom(markoReleaseLatestURL, exe)
}

func updateMarkoFrom(latestURL, exePath string) error {
	release, err := fetchLatestRelease(latestURL)
	if err != nil {
		return err
	}
	assetName, err := platformAssetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}
	asset, ok := selectReleaseAsset(release.Assets, assetName)
	if !ok {
		return fmt.Errorf("no release asset for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	data, err := downloadReleaseAsset(asset.BrowserDownloadURL)
	if err != nil {
		return err
	}
	if err := replaceExecutable(exePath, data); err != nil {
		return err
	}
	fmt.Printf("Updated to %s\n", release.TagName)
	return nil
}

func fetchLatestRelease(url string) (*githubRelease, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "marko-updater")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = resp.Status
		}
		return nil, fmt.Errorf("fetch latest release: %s", msg)
	}
	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}
	return &release, nil
}

func selectReleaseAsset(assets []githubAsset, name string) (githubAsset, bool) {
	for _, asset := range assets {
		if asset.Name == name {
			return asset, true
		}
	}
	return githubAsset{}, false
}

func platformAssetName(goos, goarch string) (string, error) {
	switch goos {
	case "darwin":
		switch goarch {
		case "arm64", "amd64":
			return "marko-darwin-" + goarch, nil
		}
	case "linux":
		switch goarch {
		case "arm64", "amd64":
			return "marko-linux-" + goarch, nil
		}
	case "windows":
		if goarch == "amd64" {
			return "marko-windows-amd64.exe", nil
		}
	}
	return "", fmt.Errorf("unsupported platform %s/%s", goos, goarch)
}

func downloadReleaseAsset(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "marko-updater")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = resp.Status
		}
		return nil, fmt.Errorf("download release asset: %s", msg)
	}
	return io.ReadAll(resp.Body)
}

func replaceExecutable(exePath string, data []byte) error {
	dir := filepath.Dir(exePath)
	tmp, err := os.CreateTemp(dir, filepath.Base(exePath)+".update-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}
	defer cleanup()
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Chmod(0755); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, exePath); err != nil {
		return err
	}
	return nil
}
