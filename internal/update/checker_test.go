package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChecker_Check(t *testing.T) {
	tests := []struct {
		name            string
		currentVersion  string
		responseStatus  int
		responseBody    string
		wantAvailable   bool
		wantVersion     string
		wantErr         bool
	}{
		{
			name:           "update available when latest is newer",
			currentVersion: "0.1.0",
			responseStatus: http.StatusOK,
			responseBody: `{
				"tag_name": "cli/v0.2.0",
				"published_at": "2025-01-15T00:00:00Z",
				"assets": [
					{"name": "coastguard-darwin-arm64", "browser_download_url": "https://example.com/coastguard-darwin-arm64", "size": 1024},
					{"name": "coastguard-linux-amd64", "browser_download_url": "https://example.com/coastguard-linux-amd64", "size": 2048}
				]
			}`,
			wantAvailable: true,
			wantVersion:   "0.2.0",
		},
		{
			name:           "up to date when versions match",
			currentVersion: "0.2.0",
			responseStatus: http.StatusOK,
			responseBody: `{
				"tag_name": "cli/v0.2.0",
				"published_at": "2025-01-15T00:00:00Z",
				"assets": []
			}`,
			wantAvailable: false,
			wantVersion:   "0.2.0",
		},
		{
			name:           "up to date when current is newer",
			currentVersion: "0.3.0",
			responseStatus: http.StatusOK,
			responseBody: `{
				"tag_name": "cli/v0.2.0",
				"published_at": "2025-01-15T00:00:00Z",
				"assets": []
			}`,
			wantAvailable: false,
			wantVersion:   "0.2.0",
		},
		{
			name:           "API error returns error",
			currentVersion: "0.1.0",
			responseStatus: http.StatusNotFound,
			responseBody:   `{"message": "Not Found"}`,
			wantErr:        true,
		},
		{
			name:           "malformed JSON returns error",
			currentVersion: "0.1.0",
			responseStatus: http.StatusOK,
			responseBody:   `not json at all`,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.responseStatus)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer srv.Close()

			checker := &Checker{
				RepoOwner:  "test-owner",
				RepoName:   "test-repo",
				HTTPClient: srv.Client(),
			}
			// Point the checker at our test server by overriding the URL
			// via a custom transport that redirects requests.
			checker.HTTPClient.Transport = &rewriteTransport{
				base:    srv.Client().Transport,
				baseURL: srv.URL,
			}

			result, err := checker.Check(context.Background(), tt.currentVersion)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.UpdateAvailable != tt.wantAvailable {
				t.Errorf("UpdateAvailable = %v, want %v", result.UpdateAvailable, tt.wantAvailable)
			}
			if result.LatestRelease.Version != tt.wantVersion {
				t.Errorf("Version = %q, want %q", result.LatestRelease.Version, tt.wantVersion)
			}
			if result.CurrentVersion != tt.currentVersion {
				t.Errorf("CurrentVersion = %q, want %q", result.CurrentVersion, tt.currentVersion)
			}
		})
	}
}

// rewriteTransport redirects all requests to the test server.
type rewriteTransport struct {
	base    http.RoundTripper
	baseURL string
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Replace the scheme+host with our test server.
	req.URL.Scheme = "http"
	req.URL.Host = t.baseURL[len("http://"):]
	if t.base != nil {
		return t.base.RoundTrip(req)
	}
	return http.DefaultTransport.RoundTrip(req)
}

func TestStripTagPrefix(t *testing.T) {
	tests := []struct {
		tag  string
		want string
	}{
		{"cli/v0.2.0", "0.2.0"},
		{"v0.2.0", "0.2.0"},
		{"0.2.0", "0.2.0"},
		{"cli/v1.0.0-beta.1", "1.0.0-beta.1"},
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			got := stripTagPrefix(tt.tag)
			if got != tt.want {
				t.Errorf("stripTagPrefix(%q) = %q, want %q", tt.tag, got, tt.want)
			}
		})
	}
}

func TestIsNewer(t *testing.T) {
	tests := []struct {
		latest  string
		current string
		want    bool
	}{
		{"0.2.0", "0.1.0", true},
		{"0.1.0", "0.1.0", false},
		{"0.1.0", "0.2.0", false},
		{"1.0.0", "0.9.9", true},
		{"invalid", "0.1.0", false},
		{"0.1.0", "invalid", false},
	}

	for _, tt := range tests {
		name := tt.latest + "_vs_" + tt.current
		t.Run(name, func(t *testing.T) {
			got := isNewer(tt.latest, tt.current)
			if got != tt.want {
				t.Errorf("isNewer(%q, %q) = %v, want %v", tt.latest, tt.current, got, tt.want)
			}
		})
	}
}
