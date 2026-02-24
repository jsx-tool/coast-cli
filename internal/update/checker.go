package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

// Release represents a GitHub release.
type Release struct {
	Version     string
	TagName     string
	PublishedAt time.Time
	Assets      []Asset
}

// Asset represents a downloadable file attached to a release.
type Asset struct {
	Name string
	URL  string
	Size int64
}

// CheckResult contains the outcome of a version check.
type CheckResult struct {
	CurrentVersion  string
	LatestRelease   Release
	UpdateAvailable bool
}

// Checker queries the GitHub releases API for the latest version.
type Checker struct {
	RepoOwner  string
	RepoName   string
	Token      string
	HTTPClient *http.Client
}

// githubRelease is the subset of the GitHub API response we care about.
type githubRelease struct {
	TagName     string        `json:"tag_name"`
	PublishedAt time.Time     `json:"published_at"`
	Assets      []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// NewChecker creates a Checker with sensible defaults.
func NewChecker(owner, repo string) *Checker {
	return &Checker{
		RepoOwner:  owner,
		RepoName:   repo,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Check fetches the latest release and compares it to currentVersion.
func (c *Checker) Check(ctx context.Context, currentVersion string) (*CheckResult, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", c.RepoOwner, c.RepoName)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching latest release: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var gh githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&gh); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	release := Release{
		Version:     stripTagPrefix(gh.TagName),
		TagName:     gh.TagName,
		PublishedAt: gh.PublishedAt,
		Assets:      make([]Asset, len(gh.Assets)),
	}
	for i, a := range gh.Assets {
		release.Assets[i] = Asset{
			Name: a.Name,
			URL:  a.BrowserDownloadURL,
			Size: a.Size,
		}
	}

	available := isNewer(release.Version, currentVersion)

	return &CheckResult{
		CurrentVersion:  currentVersion,
		LatestRelease:   release,
		UpdateAvailable: available,
	}, nil
}

// stripTagPrefix removes a "cli/" or "cli/v" prefix from a tag, then strips
// the leading "v" so the result is a bare version like "1.2.3".
func stripTagPrefix(tag string) string {
	v := strings.TrimPrefix(tag, "cli/")
	v = strings.TrimPrefix(v, "v")
	return v
}

// isNewer returns true if latest is a higher semver than current.
// Both values should be bare versions (no "v" prefix).
func isNewer(latest, current string) bool {
	// semver package requires a "v" prefix.
	l := "v" + strings.TrimPrefix(latest, "v")
	c := "v" + strings.TrimPrefix(current, "v")

	if !semver.IsValid(l) || !semver.IsValid(c) {
		return false
	}
	return semver.Compare(l, c) > 0
}
