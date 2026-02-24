package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/coastguard/cli/internal/update"
	"github.com/coastguard/cli/internal/version"
	"github.com/spf13/cobra"
)

// nudgeMessage is set during PersistentPreRunE and printed after the command runs.
var nudgeMessage string

func main() {
	rootCmd := &cobra.Command{
		Use:   "coastguard",
		Short: "Coastguard CLI — manage your coasts.dev projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
		Version:            version.String(),
		PersistentPreRunE:  policyPreRun,
		PersistentPostRunE: policyPostRun,
	}

	rootCmd.SetVersionTemplate(fmt.Sprintf("%s\n", version.String()))
	rootCmd.AddCommand(newUpdateCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func newUpdateCmd() *cobra.Command {
	updateCmd := &cobra.Command{
		Use:   "update",
		Short: "Check for and install CLI updates",
		// Running "coastguard update" with no subcommand behaves like "check".
		RunE: runUpdateCheck,
	}

	updateCmd.AddCommand(&cobra.Command{
		Use:   "check",
		Short: "Check if a newer version is available",
		RunE:  runUpdateCheck,
	})

	updateCmd.AddCommand(&cobra.Command{
		Use:   "apply",
		Short: "Download and install the latest version",
		RunE:  runUpdateApply,
	})

	return updateCmd
}

// ghToken returns a GitHub token from GITHUB_TOKEN env or gh CLI auth.
func ghToken() string {
	if t := os.Getenv("GITHUB_TOKEN"); t != "" {
		return t
	}
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func newChecker() *update.Checker {
	c := update.NewChecker("jsx-tool", "coastguard-platform")
	c.Token = ghToken()
	return c
}

func runUpdateCheck(_ *cobra.Command, _ []string) error {
	checker := newChecker()
	result, err := checker.Check(context.Background(), version.Version)
	if err != nil {
		return fmt.Errorf("update check failed: %w", err)
	}

	fmt.Printf("Current version: v%s\n", result.CurrentVersion)
	fmt.Printf("Latest version:  v%s\n", result.LatestRelease.Version)

	if result.UpdateAvailable {
		fmt.Println("\nA new version is available! Run `coastguard update apply` to install it.")
	} else {
		fmt.Println("\nYou are up to date.")
	}

	return nil
}

func runUpdateApply(_ *cobra.Command, _ []string) error {
	checker := newChecker()
	updater := update.NewUpdater(checker)

	fmt.Println("Checking for updates...")

	newVersion, err := updater.Apply(context.Background(), version.Version)
	if err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	fmt.Printf("Updated successfully to v%s!\n", newVersion)
	return nil
}

// isUpdateCommand walks up the command tree to check if we're running an update subcommand.
func isUpdateCommand(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if c.Name() == "update" {
			return true
		}
	}
	return false
}

// policyPreRun enforces the repo-hosted update policy before each command.
// Network failures fail open — the command runs regardless.
func policyPreRun(cmd *cobra.Command, _ []string) error {
	if isUpdateCommand(cmd) {
		return nil
	}

	ctx := cmd.Context()
	token := ghToken()

	policy, err := update.FetchPolicy(ctx, "jsx-tool", "coastguard-platform", "main", token, nil)
	if err != nil {
		return nil // fail open
	}

	currentVersion := version.Version

	switch policy.Policy {
	case "required":
		if update.IsBelow(currentVersion, policy.MinimumVersion) {
			msg := fmt.Sprintf("Error: CLI version v%s is below the required minimum v%s.", currentVersion, policy.MinimumVersion)
			if policy.Message != "" {
				msg += "\n" + policy.Message
			}
			msg += "\nRun `coastguard update apply` to update."
			fmt.Fprintln(os.Stderr, msg)
			os.Exit(1)
		}

	case "auto":
		checker := newChecker()
		result, err := checker.Check(ctx, currentVersion)
		if err != nil {
			return nil // fail open
		}
		if result.UpdateAvailable {
			updater := update.NewUpdater(checker)
			newVer, err := updater.Apply(ctx, currentVersion)
			if err != nil {
				return nil // fail open
			}
			fmt.Fprintf(os.Stderr, "Auto-updated to v%s, restarting...\n", newVer)
			exe, err := os.Executable()
			if err != nil {
				return nil // fail open
			}
			// Replace the current process with the updated binary.
			if err := syscall.Exec(exe, os.Args, os.Environ()); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not restart after update: %v\n", err)
			}
		}

	case "nudge":
		checker := newChecker()
		result, err := checker.Check(ctx, currentVersion)
		if err != nil {
			return nil // fail open
		}
		if result.UpdateAvailable {
			nudgeMessage = fmt.Sprintf("Update available: v%s → v%s. Run `coastguard update apply` to update.", currentVersion, result.LatestRelease.Version)
			if policy.Message != "" {
				nudgeMessage = policy.Message + " " + nudgeMessage
			}
		}
	}

	return nil
}

// policyPostRun prints a nudge message after the command completes, if one was set.
func policyPostRun(_ *cobra.Command, _ []string) error {
	if nudgeMessage != "" {
		fmt.Fprintln(os.Stderr, nudgeMessage)
	}
	return nil
}
