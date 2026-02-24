package update

import (
	"testing"
)

func TestFindAsset(t *testing.T) {
	assets := []Asset{
		{Name: "coastguard-darwin-arm64", URL: "https://example.com/coastguard-darwin-arm64", Size: 1024},
		{Name: "coastguard-darwin-amd64", URL: "https://example.com/coastguard-darwin-amd64", Size: 1024},
		{Name: "coastguard-linux-amd64", URL: "https://example.com/coastguard-linux-amd64", Size: 2048},
		{Name: "coastguard-linux-arm64", URL: "https://example.com/coastguard-linux-arm64", Size: 2048},
		{Name: "checksums.txt", URL: "https://example.com/checksums.txt", Size: 256},
	}

	tests := []struct {
		name      string
		goos      string
		goarch    string
		wantName  string
		wantErr   bool
	}{
		{
			name:     "darwin arm64",
			goos:     "darwin",
			goarch:   "arm64",
			wantName: "coastguard-darwin-arm64",
		},
		{
			name:     "darwin amd64",
			goos:     "darwin",
			goarch:   "amd64",
			wantName: "coastguard-darwin-amd64",
		},
		{
			name:     "linux amd64",
			goos:     "linux",
			goarch:   "amd64",
			wantName: "coastguard-linux-amd64",
		},
		{
			name:     "linux arm64",
			goos:     "linux",
			goarch:   "arm64",
			wantName: "coastguard-linux-arm64",
		},
		{
			name:    "windows amd64 not available",
			goos:    "windows",
			goarch:  "amd64",
			wantErr: true,
		},
		{
			name:    "linux 386 not available",
			goos:    "linux",
			goarch:  "386",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asset, err := FindAsset(assets, tt.goos, tt.goarch)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if asset.Name != tt.wantName {
				t.Errorf("asset.Name = %q, want %q", asset.Name, tt.wantName)
			}
		})
	}
}

func TestFindAsset_EmptyList(t *testing.T) {
	_, err := FindAsset(nil, "darwin", "arm64")
	if err == nil {
		t.Fatal("expected error for empty asset list, got nil")
	}
}
