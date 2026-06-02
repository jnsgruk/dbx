package main

import (
	"slices"
	"testing"
)

func TestCreateFlagsOptsDefaultTools(t *testing.T) {
	opts, err := (&createFlags{}).opts()
	if err != nil {
		t.Fatalf("opts() returned error: %v", err)
	}
	want := []string{"codex", "opencode"}
	if !slices.Equal(opts.Tools, want) {
		t.Errorf("Tools = %v, want %v", opts.Tools, want)
	}
}

func TestCreateFlagsOptsAddsToolsAfterDefaults(t *testing.T) {
	opts, err := (&createFlags{tools: "k8s"}).opts()
	if err != nil {
		t.Fatalf("opts() returned error: %v", err)
	}
	want := []string{"codex", "opencode", "k8s"}
	if !slices.Equal(opts.Tools, want) {
		t.Errorf("Tools = %v, want %v", opts.Tools, want)
	}
}

func TestCreateFlagsOptsDedupesTools(t *testing.T) {
	opts, err := (&createFlags{tools: "opencode,k8s,codex"}).opts()
	if err != nil {
		t.Fatalf("opts() returned error: %v", err)
	}
	want := []string{"codex", "opencode", "k8s"}
	if !slices.Equal(opts.Tools, want) {
		t.Errorf("Tools = %v, want %v", opts.Tools, want)
	}
}

func TestCreateFlagsOptsRejectsUnknownAdditionalTools(t *testing.T) {
	if _, err := (&createFlags{tools: "not-a-tool"}).opts(); err == nil {
		t.Fatal("opts() should reject unknown additional tools")
	}
}
