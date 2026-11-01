package modes

import (
	"encoding/json"
	"runtime"
	"strings"
	"testing"

	"github.com/patriceckhart/zot/packages/core"
	"github.com/patriceckhart/zot/packages/tui"
)

func TestShellPrefixContextFromToolEvents(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell display")
	}
	args := json.RawMessage(`{"command":"cd /repo && pwd"}`)
	for _, streaming := range []bool{false, true} {
		i := &Interactive{
			cfg:       InteractiveConfig{CWD: "/repo"},
			view:      &tui.View{Theme: tui.Dark},
			toolCalls: map[string]*tui.ToolCallView{},
			toolGate:  map[string]int{},
		}
		d := &btwDialog{cwd: "/repo", toolView: &tui.View{Theme: tui.Dark}, turns: []btwTurn{{}}}
		if streaming {
			start := core.EvToolUseStart{ID: "call", Name: "bash"}
			i.handleEvent(start)
			d.handleAgentEvent(0, start)
			// A subsequent host change must not change the observed call's cwd.
			i.cfg.CWD = "/other"
			d.cwd = "/other"
		}
		call := core.EvToolCall{ID: "call", Name: "bash", Args: args}
		i.handleEvent(call)
		d.handleAgentEvent(0, call)
		for _, output := range [][]string{
			i.view.RenderToolCall(*i.toolCalls["call"], 100),
			d.toolView.RenderToolCall(d.turns[0].Tools[0], 100),
		} {
			rendered := strings.Join(output, "\n")
			if !strings.Contains(rendered, tui.Dark.FG256(tui.Dark.Muted, "cd /repo && ")) {
				t.Fatalf("missing call-time cwd styling (streaming=%v): %q", streaming, rendered)
			}
		}
	}
}
