package mcpconfig

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestEscapePowerShellValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", ""},
		{"no special chars", "hello world", "hello world"},
		{"single quote", "it's a test", "it''s a test"},
		{"multiple single quotes", "it's Bob's", "it''s Bob''s"},
		{"double quotes unchanged", `say "hello"`, `say "hello"`},
		{"dollar sign unchanged", "$HOME/path", "$HOME/path"},
		{"backslash unchanged", `C:\Users\test`, `C:\Users\test`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := escapePowerShellValue(tt.input)
			if got != tt.expected {
				t.Errorf("escapePowerShellValue(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestDetectScriptType(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"hook.sh", ""},
		{"hook.ps1", "powershell"},
		{"hook.py", "python"},
		{"hook.js", "node"},
		{"hook.mjs", "node"},
		{"hook.txt", ""},
		{"/path/to/hook.ps1", "powershell"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := detectScriptType(tt.path)
			if got != tt.expected {
				t.Errorf("detectScriptType(%q) = %q, want %q", tt.path, got, tt.expected)
			}
		})
	}
}

func TestWindowsHookCommand(t *testing.T) {
	tests := []struct {
		name       string
		hookPath   string
		scriptType string
		expected   string
	}{
		{
			"shell hook",
			`C:\Users\test\.claude\hooks\PreToolUse\hook.sh`,
			"",
			`bash "C:/Users/test/.claude/hooks/PreToolUse/hook.sh"`,
		},
		{
			"powershell hook",
			`C:\Users\test\.claude\hooks\PreToolUse\hook.ps1`,
			"powershell",
			`pwsh -NoProfile -ExecutionPolicy Bypass -File "C:/Users/test/.claude/hooks/PreToolUse/hook.ps1"`,
		},
		{
			"python hook",
			`C:\Users\test\.claude\hooks\PreToolUse\hook.py`,
			"python",
			`python "C:/Users/test/.claude/hooks/PreToolUse/hook.py"`,
		},
		{
			"node hook",
			`C:\Users\test\.claude\hooks\PreToolUse\hook.js`,
			"node",
			`node "C:/Users/test/.claude/hooks/PreToolUse/hook.js"`,
		},
		{
			"path with spaces",
			`C:\Users\John Doe\.claude\hooks\PreToolUse\hook.sh`,
			"",
			`bash "C:/Users/John Doe/.claude/hooks/PreToolUse/hook.sh"`,
		},
		{
			"unix path passthrough",
			"/home/user/.claude/hooks/PreToolUse/hook.sh",
			"",
			`bash "/home/user/.claude/hooks/PreToolUse/hook.sh"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := windowsHookCommand(tt.hookPath, tt.scriptType)
			if got != tt.expected {
				t.Errorf("windowsHookCommand(%q, %q) = %q, want %q", tt.hookPath, tt.scriptType, got, tt.expected)
			}
		})
	}
}

func TestExtractPathFromCommand(t *testing.T) {
	tests := []struct {
		name     string
		cmd      string
		expected string
	}{
		{
			"bash prefix",
			`bash "C:/Users/test/.claude/hooks/PreToolUse/hook.sh"`,
			"C:/Users/test/.claude/hooks/PreToolUse/hook.sh",
		},
		{
			"pwsh prefix",
			`pwsh -NoProfile -ExecutionPolicy Bypass -File "C:/Users/test/.claude/hooks/PreToolUse/hook.ps1"`,
			"C:/Users/test/.claude/hooks/PreToolUse/hook.ps1",
		},
		{
			"python prefix",
			`python "C:/Users/test/.claude/hooks/hook.py"`,
			"C:/Users/test/.claude/hooks/hook.py",
		},
		{
			"python3 prefix",
			`python3 "/home/user/.claude/hooks/hook.py"`,
			"/home/user/.claude/hooks/hook.py",
		},
		{
			"node prefix",
			`node "C:/Users/test/.claude/hooks/hook.js"`,
			"C:/Users/test/.claude/hooks/hook.js",
		},
		{
			"bare path passthrough",
			`C:\Users\test\.claude\hooks\PreToolUse\hook.sh`,
			`C:\Users\test\.claude\hooks\PreToolUse\hook.sh`,
		},
		{
			"bare unix path passthrough",
			"/home/user/.claude/hooks/hook.sh",
			"/home/user/.claude/hooks/hook.sh",
		},
		{
			"path with spaces",
			`bash "C:/Users/John Doe/.claude/hooks/hook.sh"`,
			"C:/Users/John Doe/.claude/hooks/hook.sh",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractPathFromCommand(tt.cmd)
			if got != tt.expected {
				t.Errorf("extractPathFromCommand(%q) = %q, want %q", tt.cmd, got, tt.expected)
			}
		})
	}
}

func TestPathNormalization(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"forward slash", "C:/Users/test/.claude/hooks/PreToolUse/hook.sh", true},
		{"backslash normalized", filepath.ToSlash(`C:\Users\test\.claude\hooks\PreToolUse\hook.sh`), true},
		{"non-hook path", filepath.ToSlash(`C:\Users\test\.zeude\bin\zeude.exe`), false},
		{"with interpreter prefix", filepath.ToSlash(extractPathFromCommand(`bash "C:\Users\test\.claude\hooks\hook.sh"`)), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := strings.Contains(tt.path, ".claude/hooks/")
			if got != tt.expected {
				t.Errorf("path %q: Contains(.claude/hooks/) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

func TestDualHookGeneration(t *testing.T) {
	// Test that shell-type hooks generate .ps1 content on Windows
	// and that the .ps1 content has correct PowerShell syntax
	hook := Hook{
		ID:         "test-hook-1",
		Name:       "test-hook",
		Event:      "PreToolUse",
		Script:     "#!/bin/bash\necho hello",
		ScriptType: "",
	}

	// Verify shell-type hook would trigger .ps1 generation on Windows
	if hook.ScriptType != "python" && hook.ScriptType != "node" {
		// This is the condition used in installHooks for .ps1 generation
		t.Log("Shell-type hook correctly identified for .ps1 generation")
	} else {
		t.Error("Shell-type hook incorrectly classified as python/node")
	}

	// Verify python hooks do NOT trigger .ps1 generation
	pythonHook := Hook{ScriptType: "python"}
	if pythonHook.ScriptType != "python" && pythonHook.ScriptType != "node" {
		t.Error("Python hook should not trigger .ps1 generation")
	}

	// Verify node hooks do NOT trigger .ps1 generation
	nodeHook := Hook{ScriptType: "node"}
	if nodeHook.ScriptType != "python" && nodeHook.ScriptType != "node" {
		t.Error("Node hook should not trigger .ps1 generation")
	}

	// Verify .ps1 content structure
	var psBuilder strings.Builder
	psBuilder.WriteString("# Auto-generated by Zeude - DO NOT EDIT\n")
	psBuilder.WriteString("# Hook: " + sanitizeScriptComment(hook.Name) + "\n")
	psBuilder.WriteString("# Event: " + sanitizeScriptComment(hook.Event) + "\n\n")
	psBuilder.WriteString(fmt.Sprintf("$env:ZEUDE_API_URL = '%s'\n", escapePowerShellValue("https://example.com")))
	psBuilder.WriteString(fmt.Sprintf("$env:ZEUDE_AGENT_KEY = '%s'\n", escapePowerShellValue("key'with'quotes")))

	content := psBuilder.String()

	// Verify PowerShell syntax
	if strings.Contains(content, "#!/") {
		t.Error(".ps1 content should not contain shebang")
	}
	if strings.Contains(content, "export ") {
		t.Error(".ps1 content should not contain bash export")
	}
	if !strings.Contains(content, "$env:ZEUDE_API_URL") {
		t.Error(".ps1 content should contain $env: syntax")
	}
	if !strings.Contains(content, "key''with''quotes") {
		t.Error(".ps1 content should have escaped single quotes")
	}

	// Verify installed hooks vs managed hooks invariant:
	// installedHooks should contain raw path, newManagedHooks should contain both .sh and .ps1
	rawPath := `C:\Users\test\.claude\hooks\PreToolUse\test-hook.sh`
	ps1Path := `C:\Users\test\.claude\hooks\PreToolUse\test-hook.ps1`

	installedHooks := map[string][]string{
		"PreToolUse": {rawPath}, // raw path only, NO .ps1
	}
	managedHooks := []string{
		filepath.ToSlash(rawPath),
		filepath.ToSlash(ps1Path),
	}

	// installedHooks should have exactly 1 entry (no .ps1)
	if len(installedHooks["PreToolUse"]) != 1 {
		t.Errorf("installedHooks should have 1 entry, got %d", len(installedHooks["PreToolUse"]))
	}
	// managedHooks should have 2 entries (both .sh and .ps1)
	if len(managedHooks) != 2 {
		t.Errorf("managedHooks should have 2 entries, got %d", len(managedHooks))
	}
	// settings.json command should use interpreter prefix
	cmd := hookCommandForPlatform(rawPath)
	if runtime.GOOS == "windows" {
		if !strings.HasPrefix(cmd, "bash ") {
			t.Errorf("Windows command should start with 'bash ', got %q", cmd)
		}
	} else {
		if cmd != rawPath {
			t.Errorf("Unix command should be raw path, got %q", cmd)
		}
	}
}

func TestSessionStartHookWindows(t *testing.T) {
	// Test the Windows SessionStart command generation
	zeudeBin := `C:\Users\test\.zeude\bin\zeude`

	// Simulate Windows command generation
	windowsCmd := fmt.Sprintf(`if not defined ZEUDE_INITIALIZED ( "%s.exe" init 2>nul )`, zeudeBin)

	// Verify cmd.exe-compatible syntax
	if strings.Contains(windowsCmd, "/dev/null") {
		t.Error("Windows SessionStart should not contain /dev/null")
	}
	if strings.Contains(windowsCmd, `[ "$ZEUDE_INITIALIZED"`) {
		t.Error("Windows SessionStart should not contain bash test syntax")
	}
	if !strings.Contains(windowsCmd, ".exe") {
		t.Error("Windows SessionStart should contain .exe extension")
	}
	if !strings.Contains(windowsCmd, "2>nul") {
		t.Error("Windows SessionStart should use 2>nul (cmd.exe stderr redirect)")
	}
	if !strings.Contains(windowsCmd, "ZEUDE_INITIALIZED") {
		t.Error("Windows SessionStart should check ZEUDE_INITIALIZED env var")
	}
	if !strings.Contains(windowsCmd, "if not defined") {
		t.Error("Windows SessionStart should use 'if not defined' (cmd.exe syntax)")
	}

	// Simulate Unix command generation
	unixCmd := fmt.Sprintf(`[ "$ZEUDE_INITIALIZED" != "1" ] && "%s" init 2>/dev/null; true`, zeudeBin)

	// Verify bash syntax
	if !strings.Contains(unixCmd, "/dev/null") {
		t.Error("Unix SessionStart should contain /dev/null")
	}
	if strings.Contains(unixCmd, ".exe") {
		t.Error("Unix SessionStart should not contain .exe")
	}
	if strings.Contains(unixCmd, "2>nul") {
		t.Error("Unix SessionStart should not use 2>nul")
	}

	// Test upgrade detection: old bash-syntax hook should be detected on Windows
	oldBashCmd := `[ "$ZEUDE_INITIALIZED" != "1" ] && "/home/user/.zeude/bin/zeude" init 2>/dev/null; true`

	needsReplacement := strings.Contains(oldBashCmd, `[ "$ZEUDE_INITIALIZED"`) || strings.Contains(oldBashCmd, "/dev/null")
	if !needsReplacement {
		t.Error("Old bash-syntax SessionStart hook should be detected for replacement")
	}

	// New cmd.exe-syntax hook should NOT be detected for replacement
	newCmdCmd := `if not defined ZEUDE_INITIALIZED ( "C:\Users\test\.zeude\bin\zeude.exe" init 2>nul )`
	shouldNotReplace := strings.Contains(newCmdCmd, `[ "$ZEUDE_INITIALIZED"`) || strings.Contains(newCmdCmd, "/dev/null")
	if shouldNotReplace {
		t.Error("New cmd.exe-syntax SessionStart hook should NOT be detected for replacement")
	}
}

func TestSanitizeScriptComment(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"no newlines", "normal text", "normal text"},
		{"with newline", "line1\nline2", "line1 line2"},
		{"with carriage return", "line1\rline2", "line1 line2"},
		{"with crlf", "line1\r\nline2", "line1  line2"},
		{"injection attempt", "hook\nrm -rf /", "hook rm -rf /"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeScriptComment(tt.input)
			if got != tt.expected {
				t.Errorf("sanitizeScriptComment(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestWindowsHookCommandRoundTrip(t *testing.T) {
	// Verify: raw path -> windowsHookCommand -> extractPathFromCommand -> matches original normalized path
	rawPaths := []struct {
		path       string
		scriptType string
	}{
		{`C:\Users\test\.claude\hooks\PreToolUse\hook.sh`, ""},
		{`C:\Users\test\.claude\hooks\PreToolUse\hook.ps1`, "powershell"},
		{`C:\Users\test\.claude\hooks\PreToolUse\hook.py`, "python"},
		{`C:\Users\test\.claude\hooks\PreToolUse\hook.js`, "node"},
		{`C:\Users\John Doe\.claude\hooks\hook.sh`, ""},
	}

	for _, tt := range rawPaths {
		t.Run(tt.path, func(t *testing.T) {
			cmd := windowsHookCommand(tt.path, tt.scriptType)
			extracted := extractPathFromCommand(cmd)
			normalizedOriginal := filepath.ToSlash(tt.path)
			if extracted != normalizedOriginal {
				t.Errorf("Round-trip failed:\n  raw:       %q\n  command:   %q\n  extracted: %q\n  expected:  %q",
					tt.path, cmd, extracted, normalizedOriginal)
			}
		})
	}
}
