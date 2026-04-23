package tools

import (
	"path/filepath"
	"strings"
	"testing"
)

func opencodeMounts(t *testing.T) []Mount {
	t.Helper()
	tool, ok := Get("opencode")
	if !ok {
		t.Fatal("opencode tool not registered")
	}
	return tool.Mounts(Context{HostHome: "/home/u", InstanceHome: "/home/u"})
}

func TestOpenCodeMountsNoSharedDataDir(t *testing.T) {
	for _, m := range opencodeMounts(t) {
		if !m.File && strings.HasSuffix(m.Dest, filepath.Join(".local", "share", "opencode")) {
			t.Errorf("opencode tool includes a directory mount for opencode data dir: %s → %s", m.Source, m.Dest)
		}
	}
}

func TestOpenCodeAuthFile(t *testing.T) {
	var found bool
	for _, m := range opencodeMounts(t) {
		if m.Name != "opencodeauth" {
			continue
		}
		found = true
		if !m.File {
			t.Error("opencodeauth mount should have File=true")
		}
		if m.Mode != "600" {
			t.Errorf("opencodeauth mount Mode = %q, want %q", m.Mode, "600")
		}
		if !strings.HasSuffix(m.Source, filepath.Join(".local", "share", "opencode", "auth.json")) {
			t.Errorf("opencodeauth source = %q, expected to end with .local/share/opencode/auth.json", m.Source)
		}
		if !strings.HasSuffix(m.Dest, filepath.Join(".local", "share", "opencode", "auth.json")) {
			t.Errorf("opencodeauth dest = %q, expected to end with .local/share/opencode/auth.json", m.Dest)
		}
	}
	if !found {
		t.Error("opencode tool should include an opencodeauth file mount")
	}
}

func TestOpenCodeCfgIsDirectory(t *testing.T) {
	var found bool
	for _, m := range opencodeMounts(t) {
		if m.Name != "opencodecfg" {
			continue
		}
		found = true
		if m.File {
			t.Error("opencodecfg should be a directory mount, not a file mount")
		}
		if !strings.HasSuffix(m.Source, filepath.Join(".config", "opencode")) {
			t.Errorf("opencodecfg source = %q, expected to end with .config/opencode", m.Source)
		}
	}
	if !found {
		t.Error("opencode tool should include opencodecfg directory mount")
	}
}

func TestHiddenToolsExcludedFromNames(t *testing.T) {
	for _, hidden := range []string{"base", "vm", "tailscale"} {
		for _, name := range Names() {
			if name == hidden {
				t.Errorf("Names() should not include hidden tool %q", hidden)
			}
		}
	}
}
