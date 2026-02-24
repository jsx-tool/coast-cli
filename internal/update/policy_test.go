package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchPolicy(t *testing.T) {
	tests := []struct {
		name           string
		responseStatus int
		responseBody   string
		wantPolicy     string
		wantMinVer     string
		wantMessage    string
		wantErr        bool
	}{
		{
			name:           "nudge policy",
			responseStatus: http.StatusOK,
			responseBody:   `{"policy":"nudge","minimum_version":"0.1.0","message":""}`,
			wantPolicy:     "nudge",
			wantMinVer:     "0.1.0",
			wantMessage:    "",
		},
		{
			name:           "required policy with message",
			responseStatus: http.StatusOK,
			responseBody:   `{"policy":"required","minimum_version":"0.5.0","message":"Critical security fix"}`,
			wantPolicy:     "required",
			wantMinVer:     "0.5.0",
			wantMessage:    "Critical security fix",
		},
		{
			name:           "auto policy",
			responseStatus: http.StatusOK,
			responseBody:   `{"policy":"auto","minimum_version":"0.1.0","message":""}`,
			wantPolicy:     "auto",
			wantMinVer:     "0.1.0",
		},
		{
			name:           "API error returns error",
			responseStatus: http.StatusNotFound,
			responseBody:   `{"message":"Not Found"}`,
			wantErr:        true,
		},
		{
			name:           "malformed JSON returns error",
			responseStatus: http.StatusOK,
			responseBody:   `not json`,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify the Accept header is set for raw content.
				if got := r.Header.Get("Accept"); got != "application/vnd.github.raw+json" {
					t.Errorf("Accept header = %q, want application/vnd.github.raw+json", got)
				}
				w.WriteHeader(tt.responseStatus)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer srv.Close()

			client := srv.Client()
			client.Transport = &rewriteTransport{
				base:    srv.Client().Transport,
				baseURL: srv.URL,
			}

			policy, err := FetchPolicy(context.Background(), "test-owner", "test-repo", "main", "", client)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if policy.Policy != tt.wantPolicy {
				t.Errorf("Policy = %q, want %q", policy.Policy, tt.wantPolicy)
			}
			if policy.MinimumVersion != tt.wantMinVer {
				t.Errorf("MinimumVersion = %q, want %q", policy.MinimumVersion, tt.wantMinVer)
			}
			if policy.Message != tt.wantMessage {
				t.Errorf("Message = %q, want %q", policy.Message, tt.wantMessage)
			}
		})
	}
}

func TestFetchPolicy_AuthHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q, want %q", got, "Bearer test-token")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"policy":"nudge","minimum_version":"0.1.0","message":""}`))
	}))
	defer srv.Close()

	client := srv.Client()
	client.Transport = &rewriteTransport{
		base:    srv.Client().Transport,
		baseURL: srv.URL,
	}

	_, err := FetchPolicy(context.Background(), "owner", "repo", "main", "test-token", client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFetchPolicy_NetworkError(t *testing.T) {
	// Client pointing at a server that's already closed → connection refused.
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	srv.Close()

	client := srv.Client()
	client.Transport = &rewriteTransport{
		base:    srv.Client().Transport,
		baseURL: srv.URL,
	}

	_, err := FetchPolicy(context.Background(), "owner", "repo", "main", "", client)
	if err == nil {
		t.Fatal("expected error for network failure, got nil")
	}
}

func TestIsBelow(t *testing.T) {
	tests := []struct {
		current string
		minimum string
		want    bool
	}{
		{"0.1.0", "0.2.0", true},
		{"0.2.0", "0.2.0", false},
		{"0.3.0", "0.2.0", false},
		{"1.0.0", "0.9.9", false},
		{"0.9.9", "1.0.0", true},
		{"invalid", "0.1.0", false},
		{"0.1.0", "invalid", false},
	}

	for _, tt := range tests {
		name := tt.current + "_vs_" + tt.minimum
		t.Run(name, func(t *testing.T) {
			got := IsBelow(tt.current, tt.minimum)
			if got != tt.want {
				t.Errorf("IsBelow(%q, %q) = %v, want %v", tt.current, tt.minimum, got, tt.want)
			}
		})
	}
}
