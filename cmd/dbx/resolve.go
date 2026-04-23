package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jnsgruk/dbx/internal/config"
	"github.com/jnsgruk/dbx/internal/state"
)

func resolveInstance(args []string) (name, cwd string, err error) {
	if len(args) == 1 {
		return args[0], "", nil
	}
	st := state.LoadPruned()
	cwd, _ = os.Getwd()
	cwd, _ = filepath.EvalSymlinks(cwd)
	names := st[cwd]
	if len(names) == 0 {
		return "", "", fmt.Errorf("no instance associated with %s", cwd)
	}
	if len(names) == 1 {
		return names[0], cwd, nil
	}
	return "", "", fmt.Errorf("directory has multiple instances: %s (specify one as argument)", strings.Join(names, ", "))
}

func projectDir(cwd string) string {
	if cwd == "" {
		return ""
	}
	return filepath.Join("/home", config.User, filepath.Base(cwd))
}
