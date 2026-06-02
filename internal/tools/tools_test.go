package tools

import (
	"slices"
	"strings"
	"testing"
)

func TestNamesIncludesUserTools(t *testing.T) {
	names := Names()
	if len(names) == 0 {
		t.Fatal("Names returned empty list")
	}
	for _, want := range []string{"claude", "codex", "k8s", "nix", "opencode"} {
		if !slices.Contains(names, want) {
			t.Errorf("Names() missing %q, got %v", want, names)
		}
	}
}

func TestBaseToolReferencesUser(t *testing.T) {
	tool, ok := Get("base")
	if !ok {
		t.Fatal("base tool not registered")
	}
	script := tool.InstallScript(Context{User: "testuser", GitHubUser: "gh"})
	if !strings.Contains(script, "testuser") {
		t.Errorf("base install script does not reference the user: %s", script)
	}
}

func TestTailscaleToolInstallsPackage(t *testing.T) {
	tool, ok := Get("tailscale")
	if !ok {
		t.Fatal("tailscale tool not registered")
	}
	script := tool.InstallScript(Context{})
	if !strings.Contains(script, "apt-get install -y tailscale") {
		t.Errorf("tailscale install script missing apt install: %s", script)
	}
}

func TestGetUnknownTool(t *testing.T) {
	if _, ok := Get("nonexistent"); ok {
		t.Error("Get(\"nonexistent\") should return ok=false")
	}
	if err := Validate([]string{"nonexistent"}); err == nil {
		t.Error("Validate should reject unknown tool")
	}
}

func TestSelectIncludesAlwaysAndRequested(t *testing.T) {
	selected := Select(Context{VM: true}, []string{"claude"})
	var names []string
	for _, t := range selected {
		names = append(names, t.Name())
	}
	for _, want := range []string{"base", "vm", "tailscale", "claude"} {
		if !slices.Contains(names, want) {
			t.Errorf("Select missing %q, got %v", want, names)
		}
	}
}

func TestSelectVMExcludedForContainers(t *testing.T) {
	selected := Select(Context{VM: false}, nil)
	for _, tool := range selected {
		if tool.Name() == "vm" {
			t.Error("vm tool should not be selected when VM=false")
		}
	}
}
