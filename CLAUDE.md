# dbx

LXD development instance creator. Creates and manages containers and VMs with standard mounts and provisioning.

## Build & Test Commands

```
mise install                     # Install dev tools (one-time)
prek install                     # Install git hooks (one-time)
mise run fmt                     # Format code (gofumpt)
mise run lint                    # Lint (go vet)
mise run build                   # Build binary (go build -o dbx ./cmd/dbx)
mise run check                   # Full validation -- run before finishing any task
prek run -av                     # Run all hooks
```

## Workflow

Before finishing any task:
1. Run `prek run -av`
2. Ensure zero warnings from vet and clean formatting

## Structure

```
cmd/dbx/main.go                  # CLI entry point: cobra commands, flags, logger
internal/
  config/config.go               # Compile-time constants (user, shell, GitHub user, etc.)
  lxc/lxc.go                     # Thin wrapper around `lxc` CLI (os/exec)
  instance/
    instance.go                  # Creation orchestration, mounts, user creation, terminfo
    base.go                      # Base instance build/copy logic
  provision/
    provision.go                 # Provisioning commands and extras script execution
    scripts/
      extras/{claude,k8s,nix}.sh
  state/state.go                 # JSON state file (~/.local/share/dbx/state.json)
```

- `config` -- compile-time constants: `User`, `GitHubUser`, `Image`, `Shell`, `LoginShell`, `RemapUID`. Change these to personalise the build.
- `lxc` -- low-level command runner; all functions log at debug level (start/finish/elapsed). `Run` delegates to `RunStdin`; `Exec` delegates to `ExecStdin`. Batch operations (`ListInstances`, `ListInstanceInfo`) use a single `lxc list` call.
- `instance` -- orchestrates creation via `CreateOpts` struct. `CreateUser` sets up a custom user inside the instance (remapping the cloud-init `ubuntu` user out of the way). `EnsureRunning` provides start-on-demand. ID mapping uses the host UID rather than a hardcoded value.
- `provision` -- base provisioning is a `[]string` of commands run one at a time (not a script). Extras are embedded `.sh` files run with shebang + `set -ex` prepended. All provisioning functions accept a `user` parameter.
- `state` -- maps directories to instance names. Use `LoadPruned()` to load, prune stale entries, and save in one call. `RemoveByName()` for deletion.

## Conventions

- Only external dependency is `github.com/spf13/cobra`
- All logging via stdlib `log/slog`; debug level logs every lxc command executed
- Log messages: capitalised gerund phrases (`"Creating instance"`, `"Adding mount"`). Warn messages use the same form with an `"error"` key -- not `"Failed to X"`.
- Error messages: lowercase `"doing thing: %w"` (`"setting disk size: %w"`, `"running provisioning: %w"`). Never start with a capital letter or end with punctuation.
- Shared flag sets use `createFlags` struct with `register()`, `applyVMDefaults()`, and `opts()` methods -- do not duplicate flag definitions between commands.
- Extras scripts in `internal/provision/scripts/extras/` contain only commands (no shebang, no `set -ex`). Adding a new extra: add a `.sh` file and rebuild.
- Prefer delegating to existing functions over duplicating logic (e.g. `Run` -> `RunStdin`, `Exec` -> `ExecStdin`).
- Batch LXD queries where possible -- use `ListInstances`/`ListInstanceInfo` instead of per-instance subprocess calls.
