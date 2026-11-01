package tui

import (
	"path/filepath"
	"runtime"
	"strings"
)

// RememberShellCWD records display-only context for an observed shell call.
// Historical calls without this context retain their original styling.
func (v *View) RememberShellCWD(id, name, cwd string) {
	if v == nil || runtime.GOOS == "windows" || (name != "bash" && name != "Bash") || id == "" || !filepath.IsAbs(cwd) {
		return
	}
	if v.shellWorkingDirs == nil {
		v.shellWorkingDirs = make(map[string]string)
	}
	if _, exists := v.shellWorkingDirs[id]; !exists {
		v.shellWorkingDirs[id] = cwd
	}
}

// ForgetShellCWD drops call-time context when the host replaces the session.
func (v *View) ForgetShellCWD() {
	if v != nil {
		v.shellWorkingDirs = nil
		v.InvalidateRenderCache()
	}
}

func (v *View) forShellCall(id string) *View {
	cwd := v.shellWorkingDirs[id]
	if cwd == v.shellCWD {
		return v
	}
	copy := *v
	copy.shellCWD = cwd
	return &copy
}

// redundantCDPrefix accepts only literal absolute paths followed by &&.
// It deliberately does not evaluate expansions, escapes, or shell programs.
func redundantCDPrefix(command, cwd string) int {
	if cwd == "" || !strings.HasPrefix(command, "cd") {
		return 0
	}
	i := 2
	space := func(c byte) bool { return c == ' ' || c == '\t' }
	if i == len(command) || !space(command[i]) {
		return 0
	}
	for i < len(command) && space(command[i]) {
		i++
	}
	if i == len(command) {
		return 0
	}
	start := i
	var path string
	if command[i] == '\'' || command[i] == '"' {
		quote := command[i]
		i++
		start = i
		for i < len(command) && command[i] != quote {
			if command[i] == '\n' || command[i] == '\r' || (quote == '"' && strings.ContainsRune("$`\\", rune(command[i]))) {
				return 0
			}
			i++
		}
		if i == len(command) {
			return 0
		}
		path = command[start:i]
		i++
	} else {
		for i < len(command) && !space(command[i]) && command[i] != '&' {
			if strings.ContainsRune("\n\r\\\"'$`*?[]{}()<>|#!~\u003b", rune(command[i])) {
				return 0
			}
			i++
		}
		path = command[start:i]
	}
	// Exact spelling only. Cleaning .. or resolving symlinks could conceal a
	// meaningful directory change under the shell's logical path semantics.
	if !filepath.IsAbs(path) || path != cwd {
		return 0
	}
	for i < len(command) && space(command[i]) {
		i++
	}
	if !strings.HasPrefix(command[i:], "&&") {
		return 0
	}
	i += 2
	if i < len(command) && command[i] == '&' {
		return 0
	}
	for i < len(command) && space(command[i]) {
		i++
	}
	return i
}

func (v *View) styleShellCommand(command string, color int) string {
	end := redundantCDPrefix(command, v.shellCWD)
	if end == 0 {
		return v.Theme.FG256(color, command)
	}
	return v.Theme.FG256(v.Theme.Muted, command[:end]) + v.Theme.FG256(color, command[end:])
}
