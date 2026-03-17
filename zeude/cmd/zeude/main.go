// Package main provides the Zeude CLI tool.
// Subcommands: update, doctor, version
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/zeude/zeude/internal/autoupdate"
	"github.com/zeude/zeude/internal/executil"
	"github.com/zeude/zeude/internal/mcpconfig"
)

const (
	colorReset  = "\033[0m"
	colorBlue   = "\033[1;34m"
	colorGreen  = "\033[1;32m"
	colorYellow = "\033[1;33m"
	colorRed    = "\033[1;31m"
	colorGray   = "\033[0;90m"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	switch os.Args[1] {
	case "init":
		runInit()
	case "update":
		runUpdate()
	case "doctor":
		runDoctor()
	case "version", "-v", "--version":
		fmt.Printf("zeude %s\n", autoupdate.GetVersion())
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Zeude CLI - Claude Code telemetry & management")
	fmt.Println()
	fmt.Println("Usage: zeude <command>")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  init      Run initialization (MCP sync + update check)")
	fmt.Println("  update    Check for updates and install if available")
	fmt.Println("  doctor    Run diagnostic checks")
	fmt.Println("  version   Show version information")
	fmt.Println("  help      Show this help message")
}

func runUpdate() {
	fmt.Printf("%s[zeude]%s Checking for updates...", colorBlue, colorReset)

	version := autoupdate.GetVersion()
	if version == "dev" {
		fmt.Printf(" %s(dev build, skipped)%s\n", colorYellow, colorReset)
		return
	}

	result := autoupdate.CheckWithResult()

	if result.Error != nil {
		fmt.Printf(" %sfailed%s\n", colorRed, colorReset)
		fmt.Fprintf(os.Stderr, "Error: %v\n", result.Error)
		os.Exit(1)
	}

	if result.Updated {
		fmt.Printf(" %s✓ Updated to %s%s\n", colorGreen, result.NewVersion, colorReset)
		fmt.Println()
		fmt.Println("Run 'claude' to use the new version.")
	} else if result.NewVersionAvailable {
		fmt.Printf(" %s(update available: %s)%s\n", colorYellow, result.NewVersion, colorReset)
	} else {
		fmt.Printf(" %s✓ Already up to date (%s)%s\n", colorGreen, version, colorReset)
	}
}

// runInit performs the same initialization as the claude shim (MCP sync + update check)
// but without exec'ing the real claude binary. Designed to be called from SessionStart hooks
// so that Claude Code Desktop app sessions also get Zeude initialization.
func runInit() {
	var updateResult autoupdate.UpdateResult
	var syncResult mcpconfig.SyncResult
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		updateResult = autoupdate.CheckWithResult()
	}()
	go func() {
		defer wg.Done()
		syncResult = mcpconfig.Sync()
	}()
	wg.Wait()

	// Build status parts (same logic as claude shim)
	var statusParts []string

	if updateResult.Updated {
		statusParts = append(statusParts, fmt.Sprintf("updated to %s", updateResult.NewVersion))
	} else if updateResult.NewVersionAvailable {
		statusParts = append(statusParts, fmt.Sprintf("update available: %s", updateResult.NewVersion))
	}

	if syncResult.NoAgentKey {
		statusParts = append(statusParts, "no agent key")
	} else if syncResult.Success {
		if syncResult.HookCount > 0 {
			statusParts = append(statusParts, fmt.Sprintf("%d hooks", syncResult.HookCount))
		}
		if syncResult.SkillCount > 0 {
			statusParts = append(statusParts, fmt.Sprintf("%d skills", syncResult.SkillCount))
		}
		if syncResult.ServerCount > 0 {
			statusParts = append(statusParts, fmt.Sprintf("%d servers", syncResult.ServerCount))
		}
		if syncResult.FromCache {
			statusParts = append(statusParts, "cached")
		}
	} else {
		statusParts = append(statusParts, "sync failed")
		os.Exit(1)
	}

	if len(statusParts) > 0 {
		fmt.Fprintf(os.Stderr, "[zeude init] %s\n", fmt.Sprintf("%s", strings.Join(statusParts, ", ")))
	} else {
		fmt.Fprintf(os.Stderr, "[zeude init] ok\n")
	}
}

func runDoctor() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot get home directory\n")
		os.Exit(1)
	}

	// Try to find and exec the doctor binary
	doctorName := "zeude-doctor"
	if runtime.GOOS == "windows" {
		doctorName = "zeude-doctor.exe"
	}
	doctorPath := filepath.Join(home, ".zeude", "bin", doctorName)
	if _, err := os.Stat(doctorPath); err == nil {
		// Found zeude-doctor binary - exec it
		err = executil.Exec(doctorPath, []string{"zeude-doctor"}, os.Environ())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to exec zeude-doctor: %v\n", err)
			os.Exit(1)
		}
		return // unreachable after successful exec
	}

	// Fallback: run inline doctor checks if zeude-doctor binary not found
	fmt.Println("Zeude Doctor (inline mode)")
	fmt.Println("==========================")
	fmt.Println()

	// Check version
	version := autoupdate.GetVersion()
	fmt.Printf("%s[OK]%s Zeude version: %s\n", colorGreen, colorReset, version)

	// Check shim
	shimName := "claude"
	if runtime.GOOS == "windows" {
		shimName = "claude.exe"
	}
	shimPath := filepath.Join(home, ".zeude", "bin", shimName)
	if _, err := os.Stat(shimPath); err == nil {
		fmt.Printf("%s[OK]%s Shim installed: %s\n", colorGreen, colorReset, shimPath)
	} else {
		fmt.Printf("%s[FAIL]%s Shim not found at %s\n", colorRed, colorReset, shimPath)
	}

	// Check credentials
	credsPath := filepath.Join(home, ".zeude", "credentials")
	if _, err := os.Stat(credsPath); err == nil {
		fmt.Printf("%s[OK]%s Credentials configured\n", colorGreen, colorReset)
	} else {
		fmt.Printf("%s[WARN]%s No credentials file at %s\n", colorYellow, colorReset, credsPath)
	}

	// Check real claude
	realPath := filepath.Join(home, ".zeude", "real_binary_path")
	if data, err := os.ReadFile(realPath); err == nil {
		path := string(data)
		if _, err := os.Stat(path); err == nil {
			fmt.Printf("%s[OK]%s Real claude: %s\n", colorGreen, colorReset, path)
		} else {
			fmt.Printf("%s[FAIL]%s Real claude not found: %s\n", colorRed, colorReset, path)
		}
	} else {
		fmt.Printf("%s[FAIL]%s Real claude path not configured\n", colorRed, colorReset)
	}

	// Check hooks
	fmt.Println()
	fmt.Println("Hooks:")
	hooksDir := filepath.Join(home, ".claude", "hooks")
	if _, err := os.Stat(hooksDir); os.IsNotExist(err) {
		fmt.Printf("%s[INFO]%s No hooks directory\n", colorGray, colorReset)
	} else {
		// Read hook event directories (UserPromptSubmit, Stop, etc.)
		eventDirs, err := os.ReadDir(hooksDir)
		if err != nil {
			fmt.Printf("%s[FAIL]%s Cannot read hooks directory: %v\n", colorRed, colorReset, err)
		} else {
			hookCount := 0
			hookIssues := 0
			for _, eventDir := range eventDirs {
				if !eventDir.IsDir() {
					continue
				}
				eventPath := filepath.Join(hooksDir, eventDir.Name())
				hookFiles, err := os.ReadDir(eventPath)
				if err != nil {
					continue
				}
				for _, hookFile := range hookFiles {
					if hookFile.IsDir() {
						continue
					}
					hookPath := filepath.Join(eventPath, hookFile.Name())
					info, err := os.Stat(hookPath)
					if err != nil {
						fmt.Printf("%s[FAIL]%s %s/%s: cannot stat\n", colorRed, colorReset, eventDir.Name(), hookFile.Name())
						hookIssues++
						continue
					}
					hookCount++
					mode := info.Mode()
					// Check if executable (user execute bit)
					if runtime.GOOS != "windows" && mode&0100 == 0 {
						fmt.Printf("%s[FAIL]%s %s/%s: not executable (chmod +x needed)\n", colorRed, colorReset, eventDir.Name(), hookFile.Name())
						hookIssues++
					} else {
						fmt.Printf("%s[OK]%s %s/%s\n", colorGreen, colorReset, eventDir.Name(), hookFile.Name())
					}
				}
			}
			if hookCount == 0 {
				fmt.Printf("%s[INFO]%s No hooks installed\n", colorGray, colorReset)
			} else if hookIssues > 0 {
				if runtime.GOOS == "windows" {
					fmt.Printf("\n%s[WARN]%s %d hook(s) have issues. Try re-running: zeude sync\n", colorYellow, colorReset, hookIssues)
				} else {
					fmt.Printf("\n%s[WARN]%s %d hook(s) have issues. Run: chmod +x ~/.claude/hooks/*/*\n", colorYellow, colorReset, hookIssues)
				}
			}
			// On Windows, check if bash is available for hook execution
			if runtime.GOOS == "windows" {
				if _, err := exec.LookPath("bash"); err != nil {
					fmt.Printf("%s[WARN]%s Git Bash not found. Hooks will not execute. Install Git for Windows or set CLAUDE_CODE_GIT_BASH_PATH.\n", colorYellow, colorReset)
				}
			}
		}
	}

	fmt.Println()
	fmt.Printf("%s[INFO]%s Install zeude-doctor for full diagnostics\n", colorGray, colorReset)
}

// ForceUpdate forces an update check and install, ignoring any skip logic
func ForceUpdate() error {
	result := autoupdate.CheckWithResult()
	if result.Error != nil {
		return result.Error
	}

	if result.Updated {
		// Re-exec with new binary
		execPath, err := os.Executable()
		if err == nil {
			execPath, _ = filepath.EvalSymlinks(execPath)
			executil.Exec(execPath, os.Args, os.Environ())
		}
	}

	return nil
}
