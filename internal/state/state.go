package state

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"slices"

	"github.com/jnsgruk/dbx/internal/lxc"
	"github.com/jnsgruk/dbx/internal/tailscale"
)

// State maps absolute directory paths to LXD instance names.
type State map[string][]string

var statePath = func() string {
	dir, _ := os.UserHomeDir()
	return filepath.Join(dir, ".local", "share", "dbx", "state.json")
}

func Load() State {
	data, err := os.ReadFile(statePath())
	if err != nil {
		return State{}
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		slog.Warn("Loading state file, starting fresh", "error", err)
		return State{}
	}
	return s
}

func Save(s State) error {
	p := statePath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}

// Prune removes entries whose instances no longer exist in LXD.
// Uses a single `lxc list` call rather than per-instance checks.
func Prune(s State) {
	instances, err := lxc.ListInstances()
	if err != nil {
		slog.Warn("Listing instances for pruning", "error", err)
		return
	}
	liveInstances := make(map[string]struct{})
	for _, names := range s {
		for _, name := range names {
			if _, ok := instances[name]; ok {
				liveInstances[name] = struct{}{}
			}
		}
	}
	tailscale.PruneDevices(liveInstances)

	for dir, names := range s {
		alive := slices.DeleteFunc(slices.Clone(names), func(name string) bool {
			if _, ok := instances[name]; !ok {
				slog.Debug("Pruning stale instance", "dir", dir, "name", name)
				return true
			}
			return false
		})
		if len(alive) == 0 {
			delete(s, dir)
		} else {
			s[dir] = alive
		}
	}
}

// LoadPruned loads state, prunes stale entries, saves, and returns the result.
func LoadPruned() State {
	s := Load()
	Prune(s)
	if err := Save(s); err != nil {
		slog.Warn("Saving state", "error", err)
	}
	return s
}

// RemoveByName deletes all entries matching the given instance name.
func RemoveByName(s State, name string) {
	for dir, names := range s {
		filtered := slices.DeleteFunc(slices.Clone(names), func(n string) bool { return n == name })
		if len(filtered) == 0 {
			delete(s, dir)
		} else {
			s[dir] = filtered
		}
	}
}
