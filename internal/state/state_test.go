package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveByName(t *testing.T) {
	t.Run("removes matching entries", func(t *testing.T) {
		s := State{
			"/home/user/project1": "instance-a",
			"/home/user/project2": "instance-b",
			"/home/user/project3": "instance-a",
		}
		RemoveByName(s, "instance-a")

		if len(s) != 1 {
			t.Fatalf("expected 1 entry remaining, got %d: %v", len(s), s)
		}
		if _, ok := s["/home/user/project2"]; !ok {
			t.Error("expected /home/user/project2 to remain")
		}
	})

	t.Run("no match leaves state unchanged", func(t *testing.T) {
		s := State{
			"/home/user/project1": "instance-a",
			"/home/user/project2": "instance-b",
		}
		RemoveByName(s, "instance-c")
		if len(s) != 2 {
			t.Fatalf("expected 2 entries, got %d: %v", len(s), s)
		}
	})

	t.Run("empty state is fine", func(t *testing.T) {
		s := State{}
		RemoveByName(s, "anything")
		if len(s) != 0 {
			t.Fatalf("expected 0 entries, got %d", len(s))
		}
	})
}

func withTempStatePath(t *testing.T, filename string) string {
	t.Helper()
	dir := t.TempDir()
	orig := statePath
	statePath = func() string {
		return filepath.Join(dir, filename)
	}
	t.Cleanup(func() { statePath = orig })
	return dir
}

func TestLoadMissingFile(t *testing.T) {
	withTempStatePath(t, "nonexistent/state.json")

	s := Load()
	if len(s) != 0 {
		t.Fatalf("expected empty state, got %v", s)
	}
}

func TestSaveAndLoad(t *testing.T) {
	withTempStatePath(t, "dbx/state.json")

	original := State{
		"/home/user/project1": "instance-a",
		"/home/user/project2": "instance-b",
	}
	if err := Save(original); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	loaded := Load()
	if len(loaded) != len(original) {
		t.Fatalf("expected %d entries, got %d", len(original), len(loaded))
	}
	for k, v := range original {
		if loaded[k] != v {
			t.Errorf("loaded[%q] = %q, want %q", k, loaded[k], v)
		}
	}
}

func TestLoadCorruptJSON(t *testing.T) {
	dir := withTempStatePath(t, "state.json")

	p := filepath.Join(dir, "state.json")
	if err := os.WriteFile(p, []byte("{invalid json"), 0o644); err != nil {
		t.Fatalf("writing corrupt file: %v", err)
	}

	s := Load()
	if len(s) != 0 {
		t.Fatalf("expected empty state for corrupt JSON, got %v", s)
	}
}
