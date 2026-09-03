package tools

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func codexMounts(t *testing.T) []Mount {
	t.Helper()
	tool, ok := Get("codex")
	if !ok {
		t.Fatal("codex tool not registered")
	}
	return tool.Mounts(Context{HostHome: "/home/u", InstanceHome: "/home/u"})
}

func TestCodexRegistered(t *testing.T) {
	if _, ok := Get("codex"); !ok {
		t.Fatal("codex tool not registered")
	}
	if !slices.Contains(Names(), "codex") {
		t.Errorf("Names() missing codex, got %v", Names())
	}
}

func TestCodexInstallScript(t *testing.T) {
	tool, ok := Get("codex")
	if !ok {
		t.Fatal("codex tool not registered")
	}
	want := `command -v pnpm >/dev/null || mise use -g pnpm@latest
mise use -g nodejs
mkdir -p ~/.local/bin
pnpm config set global-bin-dir ~/.local/bin
pnpm add --global @openai/codex`
	if got := tool.InstallScript(Context{}); got != want {
		t.Errorf("InstallScript() = %q, want %q", got, want)
	}
}

func TestCodexMountsOnlyAuthAndConfig(t *testing.T) {
	mounts := codexMounts(t)
	want := map[string]struct {
		found bool
		copy  bool
	}{
		filepath.Join("/home/u", ".codex", "auth.json"):   {copy: false},
		filepath.Join("/home/u", ".codex", "config.toml"): {copy: true},
	}
	if len(mounts) != len(want) {
		t.Fatalf("got %d mounts, want %d: %v", len(mounts), len(want), mounts)
	}
	for _, m := range mounts {
		entry, ok := want[m.Dest]
		if !ok {
			t.Errorf("unexpected codex mount dest: %s", m.Dest)
			continue
		}
		entry.found = true
		want[m.Dest] = entry
		if !m.File {
			t.Errorf("%s should have File=true", m.Name)
		}
		if m.Copy != entry.copy {
			t.Errorf("%s Copy = %t, want %t", m.Name, m.Copy, entry.copy)
		}
		if m.Mode != "600" {
			t.Errorf("%s Mode = %q, want %q", m.Name, m.Mode, "600")
		}
		if m.Name == "codexconfig" && m.Transform == nil {
			t.Error("codexconfig should transform the guest copy")
		}
	}
	for dest, entry := range want {
		if !entry.found {
			t.Errorf("missing codex mount dest: %s", dest)
		}
	}
}

func TestDisableNodeREPL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "without enabled key",
			input: "[mcp_servers.node_repl]\ncommand = \"/desktop/node_repl\"\n",
			want:  "[mcp_servers.node_repl]\ncommand = \"/desktop/node_repl\"\nenabled = false\n",
		},
		{
			name:  "enabled true",
			input: "[mcp_servers.node_repl]\nenabled = true # desktop only\n",
			want:  "[mcp_servers.node_repl]\nenabled = false # desktop only\n",
		},
		{
			name:  "enabled false",
			input: "[mcp_servers.node_repl]\nenabled = false\n",
			want:  "[mcp_servers.node_repl]\nenabled = false\n",
		},
		{
			name: "followed by nested env table",
			input: "[mcp_servers.node_repl]\ncommand = \"node_repl\"\n\n" +
				"[mcp_servers.node_repl.env]\nenabled = \"credential\"\nTOKEN = \"secret\"\n",
			want: "[mcp_servers.node_repl]\ncommand = \"node_repl\"\n\n" +
				"enabled = false\n[mcp_servers.node_repl.env]\nenabled = \"credential\"\nTOKEN = \"secret\"\n",
		},
		{
			name: "followed by unrelated table",
			input: "[mcp_servers.node_repl]\ncommand = \"node_repl\"\n" +
				"[mcp_servers.github]\nenabled = true\n",
			want: "[mcp_servers.node_repl]\ncommand = \"node_repl\"\n" +
				"enabled = false\n[mcp_servers.github]\nenabled = true\n",
		},
		{
			name:  "at end of file without trailing newline",
			input: "model = \"gpt-5\"\n[mcp_servers.node_repl]\nargs = []",
			want:  "model = \"gpt-5\"\n[mcp_servers.node_repl]\nargs = []\nenabled = false",
		},
		{
			name:  "no node repl table",
			input: "# keep this exactly\nmodel = \"gpt-5\"\n[mcp_servers.github]\nenabled = true\n",
			want:  "# keep this exactly\nmodel = \"gpt-5\"\n[mcp_servers.github]\nenabled = true\n",
		},
		{
			name: "similarly prefixed server",
			input: "[mcp_servers.node_repl_other]\nenabled = true\n" +
				"[mcp_servers.node_repl.extra]\nenabled = true\n",
			want: "[mcp_servers.node_repl_other]\nenabled = true\n" +
				"[mcp_servers.node_repl.extra]\nenabled = true\n",
		},
		{
			name: "preserves unrelated comments and config",
			input: "# preferred model\nmodel = \"gpt-5\"\n\n" +
				"[mcp_servers.node_repl] # from Desktop\nargs = [] # keep\n\n" +
				"# another server\n[mcp_servers.docs]\ncommand = \"docs\"\n",
			want: "# preferred model\nmodel = \"gpt-5\"\n\n" +
				"[mcp_servers.node_repl] # from Desktop\nargs = [] # keep\n\n" +
				"enabled = false\n# another server\n[mcp_servers.docs]\ncommand = \"docs\"\n",
		},
		{
			name:  "preserves CRLF trailing newline",
			input: "[mcp_servers.node_repl]\r\ncommand = \"node_repl\"\r\n",
			want:  "[mcp_servers.node_repl]\r\ncommand = \"node_repl\"\r\nenabled = false\r\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := disableNodeREPL([]byte(tt.input))
			if err != nil {
				t.Fatalf("disableNodeREPL() returned error: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("disableNodeREPL() = %q, want %q", got, tt.want)
			}
			if tt.name == "enabled false" && strings.Count(string(got), "enabled = false") != 1 {
				t.Errorf("disableNodeREPL() duplicated enabled key: %q", got)
			}
		})
	}
}

func TestDisableNodeREPLRejectsInvalidEnabledValue(t *testing.T) {
	_, err := disableNodeREPL([]byte("[mcp_servers.node_repl]\nenabled = \"yes\"\n"))
	if err == nil {
		t.Fatal("disableNodeREPL() should reject a non-boolean enabled value")
	}
}

func TestCodexMountsAvoidStateAndHistoryPaths(t *testing.T) {
	codexDir := filepath.Join("/home/u", ".codex")
	forbidden := []string{
		filepath.Join(".codex", "sessions"),
		filepath.Join(".codex", "history.jsonl"),
		filepath.Join(".codex", "cache"),
		filepath.Join(".codex", "tmp"),
		filepath.Join(".codex", "shell_snapshots"),
		filepath.Join(".codex", "memories"),
		filepath.Join(".codex", "rules"),
		filepath.Join(".codex", "skills"),
	}
	for _, m := range codexMounts(t) {
		if m.Dest == codexDir {
			t.Errorf("codex tool should not mount the .codex directory: %s", m.Dest)
		}
		for _, path := range forbidden {
			if strings.Contains(m.Dest, path) {
				t.Errorf("codex mount includes forbidden path %q: %s", path, m.Dest)
			}
		}
		if strings.Contains(filepath.Base(m.Dest), "logs_") && strings.HasSuffix(m.Dest, ".sqlite") {
			t.Errorf("codex mount includes logs sqlite state path: %s", m.Dest)
		}
	}
}
