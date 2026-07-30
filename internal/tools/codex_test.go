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
	want := `mise use -g nodejs
mise use -g npm:@openai/codex`
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
	}
	for dest, entry := range want {
		if !entry.found {
			t.Errorf("missing codex mount dest: %s", dest)
		}
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
