# dbx — agent notes

LXD development instance creator. A single-user CLI that builds, caches, and
reconnects to disposable Ubuntu containers/VMs with the current directory
mounted and standard provisioning applied.

This file is the north star for automated agents working on the repo. Read
it fully before editing; prefer delegating to existing helpers over
duplicating logic.

## Build & test commands

```
mise install                     # one-time: install pinned dev tools
prek install                     # one-time: install git hooks
mise run fmt                     # gofumpt
mise run lint                    # go vet
mise run build                   # go build -o dbx ./cmd/dbx
mise run check                   # fmt + lint + test + build (required before finishing)
prek run -av                     # run all git hooks over the tree
go test ./...                    # run tests directly
```

## Workflow

Before finishing any task:

1. Run `prek run -av` (covers `gofumpt`, `go vet`, `go test ./...`, and
   `go build`). The same checks gate commits via pre-commit hooks.
2. Ensure zero warnings from `vet` and clean `gofumpt` output.
3. Commit with [Conventional Commits](https://www.conventionalcommits.org/)
   — e.g. `feat(cli): ...`, `fix(lxc): ...`, `docs: ...`,
   `refactor(tools): ...`, `test(state): ...`. Scope in parentheses is
   usually the package or subsystem (`cli`, `lxc`, `instance`, `tools`,
   `state`, `tailscale`, `config`, `readme`, `claude`).
4. Keep commits focused; split orthogonal changes into separate commits.

## Repository layout

```
cmd/dbx/
  main.go            # entry point: main(), setupLogger, buildVersion
  root.go            # root command, createFlags, findInstance
  resolve.go         # resolveInstance, projectDir
  create.go          # 'create' subcommand
  shell.go           # 'shell', 'stop', 'rm' subcommands
  exec.go            # 'exec' subcommand (args after -- are the command)
  list.go            # 'ls' subcommand + timeAgo
  base.go            # 'base' subcommand group (ls, rm)
  tools.go           # 'tools list' subcommand
  main_test.go       # tests for version/logger helpers
internal/
  config/config.go           # compile-time constants (User, GitHubUser, Image, Shell, RemapUID, ...)
  lxc/                        # thin wrapper around the `lxc` CLI (os/exec)
    lxc.go                    # core command execution (Run/RunStdin, Exec/ExecStdin, Shell, Stop, ForceDelete, ExecInteractive, ExecScript, file push)
    info.go                   # ListInstances, ListInstanceInfo, GetInstanceInfo
    project.go                # LXD project management for the base-instance project
  instance/
    instance.go               # CreateOpts, Create, createFull, finalizeInstance, EnsureRunning
    base.go                   # base instance build/copy logic
    wait.go                   # WaitForUser/Network/Snapd/Shell + timeout constants
  tools/                      # composable, registered Tool implementations
    tools.go                  # Tool, Always, Hidden, Authenticator interfaces, registry, Validate, Names, All
    base.go                   # core provisioning (always, hidden): probuntu, ssh-import-id, sysctl, starship
    vm.go                     # VM swapfile (always when opts.VM, hidden)
    tailscale.go              # tailscale apt install (always, hidden) + Authenticate for --tailscale
    claude.go                 # opt-in: claude-code via mise + mounts
    opencode.go               # opt-in: opencode via mise + config + auth.json file mount
    k8s.go                    # opt-in: Canonical k8s snap, MetalLB, local registry, kubeconfig
    nix.go                    # opt-in: Determinate Systems Nix installer
    run.go                    # Install pipeline: iterate selected tools, exec scripts as user
  tailscale/tailscale.go      # 1Password-backed OAuth client + Tailscale API; AuthKey, PruneDevices, RemoveDeviceByInstance
  state/state.go              # JSON state (~/.local/share/dbx/state.json): Load, LoadPruned, Save, RemoveByName
```

All non-trivial packages have `_test.go` siblings; add tests alongside
new behaviour where practical.

## Subsystem notes

### `config`
Compile-time constants only. Personalise by editing and rebuilding. No
runtime config file, by design.

### `lxc`
Low-level subprocess wrapper around the `lxc` CLI. Every function logs at
debug level with start/finish/elapsed. `Run` delegates to `RunStdin`;
`Exec` delegates to `ExecStdin`; `Shell` and `ExecInteractive` handle PTY
sessions. Batch with `ListInstances` / `ListInstanceInfo` (single
`lxc list` call) rather than per-instance subprocess calls.

### `instance`
Orchestrates creation via `CreateOpts`. Key pieces:
- `GenerateName` — `<purpose>-<image-release>-<4 hex>`.
- `createFull` — launches base project build if needed, copies from base,
  configures resource limits, attaches mounts, starts, waits for
  readiness, runs tool install scripts, handles Tailscale auth.
- `CreateUser` — remaps the cloud-init `ubuntu` user to `config.RemapUID`
  then creates the target user matching the host UID with passwordless
  sudo.
- `EnsureRunning` — start-on-demand helper used by `shell`, `exec`.
- `wait.go` — centralised poll helpers; `pollInterval = 200ms`; per-gate
  timeouts are exported constants (`UserWaitTimeout`, `NetworkWaitTimeout`,
  `SnapdWaitTimeout`, `ShellWaitTimeout`).

### `tools`
Everything installable or mountable in an instance is a `Tool`. The
interface:

```go
type Tool interface {
    Name() string
    Description() string
    Mounts(Context) []Mount
    InstallScript(Context) string  // "" means no install step
}
```

Optional extensions: `Always(Context) bool` (selected unconditionally when
true), `Hidden() bool` (omit from `dbx tools list`),
`Authenticator.Authenticate(Context, key) (string, error)` (post-install
auth, e.g. Tailscale; returns an opaque device id to persist in state).

`Install` in `run.go` wraps each non-empty script with
`#!/usr/bin/env bash\nset -exo pipefail\n` and streams it via
`lxc.ExecScript`. Mounts with `File: true` are bind-mounted on containers
and `lxc file push`'d on VMs (VMs can't bind-mount single files). Use
`Build: true` for mounts that must exist during the base image build.

#### Adding a tool

1. Create `internal/tools/<name>.go` with a struct implementing `Tool`
   (and optionally `Always`/`Hidden`/`Authenticator`).
2. `func init() { Register(<name>{}) }` to register.
3. Add it to `--tools` in flag help implicitly via `tools.Names()`.
4. Add a test in `internal/tools/<name>_test.go` where behaviour warrants
   (see `opencode_test.go` for patterns).

No file edits are needed elsewhere; registration is the contract.

### `tailscale`
Uses the `op` 1Password CLI (subprocess, not a Go dep) to fetch an OAuth
client secret and an API token. Mints ephemeral, pre-authorised auth keys
via `tailscale-client-go`. `PruneDevices(live)` removes device records for
instances no longer in LXD; `RemoveDeviceByInstance(name)` is called from
`dbx rm`.

### `state`
JSON map of host directory → list of instance names, at
`~/.local/share/dbx/state.json`. Always prefer `LoadPruned()` which loads,
drops stale entries (instances no longer in LXD), and returns both the
pruned map and the set of live instance names (used to prune Tailscale
devices in the same pass). `RemoveByName(st, name)` for targeted removal.

## Conventions

- **Dependencies.** `spf13/cobra` + `spf13/pflag` (CLI),
  `tailscale/tailscale-client-go` (Tailscale API). The `op` 1Password CLI
  is a subprocess, not a Go dependency.
- **Logging.** stdlib `log/slog` only. Debug level logs every `lxc`
  command executed. Message style: capitalised gerund phrases
  (`"Creating instance"`, `"Adding mount"`). Warnings use the same form
  with an `"error"` key — never `"Failed to X"`.
- **Errors.** Lowercase, `"doing thing: %w"`
  (`"setting disk size: %w"`, `"running provisioning: %w"`). Never start
  with a capital letter; never end with punctuation.
- **Cobra.** Root command sets `SilenceUsage: true` so usage is only
  printed for flag/arg parsing errors, not for `RunE`-returned errors.
  Keep it that way.
- **Flag sharing.** Creation flags live in the `createFlags` struct
  (`register`, `applyVMDefaults`, `opts`). Do not duplicate flag
  definitions between the root and `create` commands.
- **Delegation.** Prefer existing helpers (`Run` → `RunStdin`,
  `Exec` → `ExecStdin`, `LoadPruned` over `Load`+manual prune). Batch
  LXD queries where possible.
- **Tools over ad-hoc scripts.** New provisioning belongs in
  `internal/tools/`, not inline in `instance.go`.
- **No secrets in code.** Anything sensitive (Tailscale OAuth, API token)
  goes through the 1Password CLI at runtime.

## Testing posture

- Pure functions and formatting helpers (`timeAgo`, `imageBase`,
  `findInstance`, state manipulation, tool validation) are unit-testable
  and should have tests.
- Anything that shells out to `lxc` or `op` is not exercised in CI; keep
  those layers thin and push logic into testable helpers.
- `go test ./...` must pass; it runs under `prek`.

## Release

Version is baked in via `-ldflags` at build time (`version`, `commit`,
`date` vars in `cmd/dbx/main.go`). Nothing release-specific lives in code
beyond that.
