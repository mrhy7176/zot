package tui

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/patriceckhart/zot/packages/provider"
)

func TestRedundantCDPrefix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX literal paths")
	}
	for _, tc := range []struct {
		command string
		cwd     string
		prefix  string
	}{
		{"cd /repo && pwd", "/repo", "cd /repo && "},
		{"cd '/my repo' && pwd", "/my repo", "cd '/my repo' && "},
		{"cd \"/my repo\"&&pwd", "/my repo", "cd \"/my repo\"&&"},
		{"cd\t/repo\t&&\tpwd", "/repo", "cd\t/repo\t&&\t"},
		{"cd /repo &&\npwd", "/repo", "cd /repo &&"},
		{"cd /repo &&", "/repo", "cd /repo &&"},
		{"cd /elsewhere && pwd", "/repo", ""},
		{"cd /repo-child && pwd", "/repo", ""},
		{"cd /repo && pwd", "", ""},
		{"cd . && pwd", "/repo", ""},
		{"cd /repo/../repo && pwd", "/repo", ""},
		{"cd /repo/ && pwd", "/repo", ""},
		{"cd $PWD && pwd", "/repo", ""},
		{"cd \"/$NAME\" && pwd", "/$NAME", ""},
		{"cd /re* && pwd", "/re*", ""},
		{"cd /re\\po && pwd", "/repo", ""},
		{"cd /repo || pwd", "/repo", ""},
		{"cd /repo & pwd", "/repo", ""},
		{"cd /repo &&& pwd", "/repo", ""},
		{"cd '/literal$dir' && pwd", "/literal$dir", "cd '/literal$dir' && "},
		{"cd \"/repo`pwd`\" && pwd", "/repo`pwd`", ""},
		{"cd /repo", "/repo", ""},
		{"cd '/repo", "/repo", ""},
		{"cd \"/repo\"x && pwd", "/repo", ""},
		{"cd /repo\npwd", "/repo", ""},
		{"pwd && cd /repo && pwd", "/repo", ""},
		{" cd /repo && pwd", "/repo", ""},
		{"cd -- /repo && pwd", "/repo", ""},
		{"cd", "/repo", ""},
		{"cd ", "/repo", ""},
		{"", "/repo", ""},
	} {
		t.Run(tc.command+tc.cwd, func(t *testing.T) {
			end := redundantCDPrefix(tc.command, tc.cwd)
			if got := tc.command[:end]; got != tc.prefix {
				t.Fatalf("prefix = %q, want %q", got, tc.prefix)
			}
		})
	}
}

func TestShellPrefixRendering(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows bash tool runs cmd")
	}
	command := "cd /repo && printf hello\nprintf goodbye"
	args, err := json.Marshal(map[string]string{"command": command})
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"bordered", "flat", "compact", "collapsed"} {
		for _, width := range []int{28, 100} {
			t.Run(fmt.Sprintf("%s/%d", mode, width), func(t *testing.T) {
				v := View{Theme: Dark, FlatTools: mode == "flat", CompactMode: mode == "compact", CollapseToolCall: mode == "collapsed"}
				v.RememberShellCWD("call", "bash", "/repo")
				// Later directory changes must not restyle an old call.
				v.RememberShellCWD("call", "bash", "/other")
				v.RememberShellCWD("new", "bash", "/other")
				tc := ToolCallView{ID: "call", Name: "bash", RawJSONBuf: string(args)}
				live := strings.Join(v.RenderToolCall(tc, width), "\n")
				result := "$ " + command + "\n\nhello\ngoodbye\n\n[exit 0]"
				tc.Done = true
				tc.Result = result
				completed := strings.Join(v.RenderToolCall(tc, width), "\n")
				v.Messages = []provider.Message{
					{Role: provider.RoleAssistant, Content: []provider.Content{provider.ToolCallBlock{ID: "call", Name: "bash", Arguments: args}}},
					{Role: provider.RoleTool, Content: []provider.Content{provider.ToolResultBlock{CallID: "call", Content: []provider.Content{provider.TextBlock{Text: result}}}}},
				}
				transcript := strings.Join(v.Build(width), "\n")
				for _, rendered := range []string{live, completed, transcript} {
					// Collapsed previews intentionally retain only the final row.
					mutedStart := strings.TrimSuffix(v.Theme.FG256(v.Theme.Muted, "cd /repo"), "\x1b[0m")
					if mode != "collapsed" && !strings.Contains(rendered, mutedStart) {
						t.Fatalf("missing muted prefix: %q", rendered)
					}
					for _, line := range strings.Split(rendered, "\n") {
						if visibleWidth(line) > width {
							t.Fatalf("row exceeds width %d: %q", width, line)
						}
					}
				}
				// Plain text and execution inputs must be unchanged.
				if tc.RawJSONBuf != string(args) || tc.Result != result {
					t.Fatal("rendering modified tool data")
				}
				unknown := View{Theme: Dark}
				plainLive := stripANSI(strings.Join(unknown.RenderToolCall(ToolCallView{ID: "call", Name: "bash", RawJSONBuf: string(args)}, width), "\n"))
				if stripANSI(live) != plainLive && mode == "bordered" {
					t.Fatal("prefix styling changed visible command text")
				}
			})
		}
	}
}

func TestShellPrefixIncompleteArgumentsAndUnknownContext(t *testing.T) {
	v := View{Theme: Dark}
	v.RememberShellCWD("bash", "bash", "/repo")
	v.RememberShellCWD("ps", "powershell", "/repo")
	v.RememberShellCWD("custom", "custom", "/repo")
	for _, id := range []string{"unknown", "ps", "custom"} {
		if v.forShellCall(id).shellCWD != "" {
			t.Fatalf("unexpected shell context for %s", id)
		}
	}
	for _, raw := range []string{`{"command":"cd /repo`, `{"command":"cd '/repo`, `{"command":"cd /repo &`} {
		rendered := strings.Join(v.RenderToolCall(ToolCallView{ID: "bash", Name: "bash", RawJSONBuf: raw, Streaming: true}, 100), "\n")
		if strings.Contains(rendered, v.Theme.FG256(v.Theme.Muted, "cd ")) {
			t.Fatalf("dimmed incomplete prefix: %q", rendered)
		}
	}
	if runtime.GOOS == "windows" && len(v.shellWorkingDirs) != 0 {
		t.Fatal("registered POSIX styling on Windows")
	}
	v.ForgetShellCWD()
	if len(v.shellWorkingDirs) != 0 || v.forShellCall("bash").shellCWD != "" {
		t.Fatal("retained context after session replacement")
	}
}

func TestShellPrefixStylesOnlyPrefix(t *testing.T) {
	v := View{Theme: Dark, shellCWD: "/repo"}
	command := "cd /repo && printf hello"
	got := v.styleShellCommand(command, v.Theme.Accent)
	want := v.Theme.FG256(v.Theme.Muted, "cd /repo && ") + v.Theme.FG256(v.Theme.Accent, "printf hello")
	if runtime.GOOS == "windows" {
		want = v.Theme.FG256(v.Theme.Accent, command)
	}
	if got != want || stripANSI(got) != command {
		t.Fatalf("styled command = %q, want %q", got, want)
	}
}
