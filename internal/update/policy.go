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

// Policy represents the CLI update policy fetched from the repo.
type Policy struct {
	Policy         string `json:"policy"`
	MinimumVersion string `json:"minimum_version"`
	Message        string `json:"message"`
}

// FetchPolicy retrieves the update policy JSON from the GitHub repo.
func FetchPolicy(ctx context.Context, owner, repo, branch, token string, httpClient *http.Client) (*Policy, error) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 5 * time.Second}
	}

	url := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/contents/cli-update-policy.json?ref=%s",
		owner, repo, branch,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.raw+json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching policy: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var p Policy
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return nil, fmt.Errorf("decoding policy: %w", err)
	}

	return &p, nil
}

// IsBelow returns true if current is a lower semver than minimum.
// Both values should be bare versions (no "v" prefix).
func IsBelow(current, minimum string) bool {
	c := "v" + strings.TrimPrefix(current, "v")
	m := "v" + strings.TrimPrefix(minimum, "v")

	if !semver.IsValid(c) || !semver.IsValid(m) {
		return false
	}
	return semver.Compare(c, m) < 0
}
