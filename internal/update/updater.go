package update

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"time"
)

// Updater downloads and installs new CLI releases.
type Updater struct {
	Checker    *Checker
	HTTPClient *http.Client
}

// NewUpdater creates an Updater backed by the given Checker.
func NewUpdater(checker *Checker) *Updater {
	return &Updater{
		Checker:    checker,
		HTTPClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// Apply checks for the latest release, downloads the matching binary,
// and replaces the current executable. Returns the new version string.
func (u *Updater) Apply(ctx context.Context, currentVersion string) (string, error) {
	result, err := u.Checker.Check(ctx, currentVersion)
	if err != nil {
		return "", fmt.Errorf("checking for update: %w", err)
	}
	if !result.UpdateAvailable {
		return "", fmt.Errorf("already up to date (v%s)", currentVersion)
	}

	asset, err := FindAsset(result.LatestRelease.Assets, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", err
	}

	tmpFile, err := u.download(ctx, asset.URL)
	if err != nil {
		return "", fmt.Errorf("downloading release: %w", err)
	}
	defer func() { _ = os.Remove(tmpFile) }() // clean up on any path

	if err := u.replaceBinary(tmpFile); err != nil {
		return "", fmt.Errorf("replacing binary: %w", err)
	}

	return result.LatestRelease.Version, nil
}

// FindAsset selects the binary asset matching the given OS and architecture.
// Asset names are expected to follow the pattern "coastguard-{os}-{arch}".
func FindAsset(assets []Asset, goos, goarch string) (*Asset, error) {
	target := fmt.Sprintf("coastguard-%s-%s", goos, goarch)
	for i := range assets {
		if assets[i].Name == target {
			return &assets[i], nil
		}
	}
	return nil, fmt.Errorf("no binary found for %s/%s", goos, goarch)
}

// download fetches the asset URL into a temporary file and returns its path.
func (u *Updater) download(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := u.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	tmp, err := os.CreateTemp("", "coastguard-update-*")
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return "", fmt.Errorf("writing temp file: %w", err)
	}
	_ = tmp.Close()

	// Make the downloaded binary executable.
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		_ = os.Remove(tmp.Name())
		return "", fmt.Errorf("setting permissions: %w", err)
	}

	return tmp.Name(), nil
}

// replaceBinary performs an atomic-ish swap of the running executable.
// Steps: rename current -> .old, rename new -> current, remove .old.
func (u *Updater) replaceBinary(newBinaryPath string) error {
	currentPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locating current executable: %w", err)
	}

	oldPath := currentPath + ".old"

	// Move current binary out of the way.
	if err := os.Rename(currentPath, oldPath); err != nil {
		return fmt.Errorf("backing up current binary: %w", err)
	}

	// Move new binary into place.
	if err := os.Rename(newBinaryPath, currentPath); err != nil {
		// Try to restore the old binary.
		_ = os.Rename(oldPath, currentPath)
		return fmt.Errorf("installing new binary: %w", err)
	}

	// Best-effort cleanup of the old binary.
	_ = os.Remove(oldPath)

	return nil
}
