package provision

import (
	"slices"
	"strings"
	"testing"
)

func TestListExtras(t *testing.T) {
	extras := ListExtras()
	if len(extras) == 0 {
		t.Fatal("ListExtras returned empty list")
	}

	expected := []string{"claude", "k8s", "nix", "opencode"}
	for _, name := range expected {
		if !slices.Contains(extras, name) {
			t.Errorf("ListExtras() missing %q, got %v", name, extras)
		}
	}
}

func TestBaseCommands(t *testing.T) {
	cmds := baseCommands("testuser")
	if len(cmds) == 0 {
		t.Fatal("baseCommands returned empty list")
	}

	found := false
	for _, cmd := range cmds {
		if strings.Contains(cmd, "testuser") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("baseCommands(\"testuser\") does not reference the user, got %v", cmds)
	}

	if !slices.Contains(cmds, `sudo apt-get update && sudo apt-get install -y tailscale`) {
		t.Errorf("baseCommands() missing tailscale install command, got %v", cmds)
	}
}

func TestGetExtra(t *testing.T) {
	t.Run("known extra returns content", func(t *testing.T) {
		for _, name := range ListExtras() {
			content, err := getExtra(name)
			if err != nil {
				t.Errorf("getExtra(%q) returned error: %v", name, err)
				continue
			}
			if content == "" {
				t.Errorf("getExtra(%q) returned empty content", name)
			}
		}
	})

	t.Run("unknown extra returns error", func(t *testing.T) {
		_, err := getExtra("nonexistent")
		if err == nil {
			t.Error("getExtra(\"nonexistent\") should return error")
		}
	})
}
