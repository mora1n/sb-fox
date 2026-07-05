package manage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
)

const (
	defaultLatestURL      = "https://api.github.com/repos/mora1n/sb-fox/releases/latest"
	defaultDownloadBase   = "https://api.github.com/repos/mora1n/sb-fox/releases/assets"
	privateRepoUpdateHint = "hint: export SB_FOX_GITHUB_TOKEN=...\n      sb-fox -u"
)

type releaseInfo struct {
	TagName string         `json:"tag_name"`
	Assets  []releaseAsset `json:"assets"`
}

type releaseAsset struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func fetchLatest(opts Options) (releaseInfo, error) {
	var latest releaseInfo
	req, err := newReleaseRequest(opts, http.MethodGet, opts.LatestURL, "application/vnd.github+json")
	if err != nil {
		return latest, errors.New("release metadata unavailable")
	}
	resp, err := opts.HTTPClient.Do(req)
	if err != nil {
		return latest, errors.New("release metadata unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return latest, releaseMetadataStatusError(resp.StatusCode, opts.GitHubToken != "")
	}
	if err := json.NewDecoder(resp.Body).Decode(&latest); err != nil {
		return latest, errors.New("release metadata unavailable (invalid JSON)")
	}
	if latest.TagName == "" {
		return latest, errors.New("release metadata missing version")
	}
	return latest, nil
}

func releaseTokenFromEnv() string {
	if token := strings.TrimSpace(os.Getenv("SB_FOX_GITHUB_TOKEN")); token != "" {
		return token
	}
	return strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
}

func newReleaseRequest(opts Options, method, url, accept string) (*http.Request, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("User-Agent", "sb-fox")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if opts.GitHubToken != "" {
		req.Header.Set("Authorization", "Bearer "+opts.GitHubToken)
	}
	return req, nil
}

func releaseAssetByName(assets []releaseAsset, name string) (releaseAsset, error) {
	for _, asset := range assets {
		if asset.Name != name {
			continue
		}
		if asset.ID == 0 {
			return releaseAsset{}, fmt.Errorf("release asset %q missing id", name)
		}
		return asset, nil
	}
	return releaseAsset{}, fmt.Errorf("release asset %q not found", name)
}

func releaseMetadataStatusError(status int, hasToken bool) error {
	if releaseAuthStatus(status) {
		if hasToken {
			return fmt.Errorf("release metadata unavailable; check SB_FOX_GITHUB_TOKEN permission or release availability (HTTP %d)\n%s", status, privateRepoUpdateHint)
		}
		return errors.New("release metadata unavailable; private repository requires SB_FOX_GITHUB_TOKEN with Contents read permission\n" + privateRepoUpdateHint)
	}
	return fmt.Errorf("release metadata unavailable (HTTP %d)", status)
}

func releaseDownloadStatusError(label string, status int, hasToken bool) error {
	if releaseAuthStatus(status) {
		if hasToken {
			return fmt.Errorf("%s failed; check SB_FOX_GITHUB_TOKEN permission or release availability (HTTP %d)\n%s", label, status, privateRepoUpdateHint)
		}
		return fmt.Errorf("%s failed; private repository requires SB_FOX_GITHUB_TOKEN with Contents read permission\n%s", label, privateRepoUpdateHint)
	}
	return fmt.Errorf("%s failed (HTTP %d)", label, status)
}

func releaseAuthStatus(status int) bool {
	return status == http.StatusUnauthorized || status == http.StatusForbidden || status == http.StatusNotFound
}

func sameReleaseVersion(current, latest string) bool {
	current = strings.TrimSpace(current)
	latest = strings.TrimSpace(latest)
	if current == "" || current == "dev" || latest == "" {
		return false
	}
	return strings.TrimPrefix(current, "v") == strings.TrimPrefix(latest, "v")
}

func releaseArchiveName(tag string) (string, error) {
	if runtime.GOOS != "linux" {
		return "", errors.New("update is only supported on Linux")
	}
	switch runtime.GOARCH {
	case "amd64", "arm64":
		return fmt.Sprintf("sb-fox-linux-%s-%s.tar.gz", runtime.GOARCH, tag), nil
	default:
		return "", fmt.Errorf("unsupported architecture %s", runtime.GOARCH)
	}
}

func downloadAsset(opts Options, assetID int64, path, label string) error {
	url := strings.TrimRight(opts.DownloadBase, "/") + "/" + strconv.FormatInt(assetID, 10)
	req, err := newReleaseRequest(opts, http.MethodGet, url, "application/octet-stream")
	if err != nil {
		return fmt.Errorf("%s failed", label)
	}
	resp, err := opts.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s failed", label)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return releaseDownloadStatusError(label, resp.StatusCode, opts.GitHubToken != "")
	}
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("%s failed", label)
	}
	defer file.Close()
	if _, err := io.Copy(file, resp.Body); err != nil {
		return fmt.Errorf("%s failed", label)
	}
	return nil
}
