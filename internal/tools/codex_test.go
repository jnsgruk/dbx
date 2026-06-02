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
	script := tool.InstallScript(Context{})
	if !strings.Contains(script, "mise use -g codex@latest") {
		t.Errorf("install script missing mise install: %s", script)
	}
}

func TestCodexMountsOnlyAuthAndConfig(t *testing.T) {
	mounts := codexMounts(t)
	want := map[string]bool{
		filepath.Join("/home/u", ".codex", "auth.json"):   false,
		filepath.Join("/home/u", ".codex", "config.toml"): false,
	}
	if len(mounts) != len(want) {
		t.Fatalf("got %d mounts, want %d: %v", len(mounts), len(want), mounts)
	}
	for _, m := range mounts {
		if _, ok := want[m.Dest]; !ok {
			t.Errorf("unexpected codex mount dest: %s", m.Dest)
			continue
		}
		want[m.Dest] = true
		if !m.File {
			t.Errorf("%s should have File=true", m.Name)
		}
		if m.Mode != "600" {
			t.Errorf("%s Mode = %q, want %q", m.Name, m.Mode, "600")
		}
	}
	for dest, found := range want {
		if !found {
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
