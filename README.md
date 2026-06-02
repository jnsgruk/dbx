# dbx

A personal tool for managing LXD-backed development environments.

`dbx` creates and reconnects to Ubuntu containers (or VMs) with the current
directory mounted, a user matching the host, GitHub SSH keys imported, and
a handful of tools provisioned. It's opinionated and single-user — I built
it for my own workflow. Sharing it in case it's useful, or as a reference
for building something similar.

## Contents

- [Prerequisites](#prerequisites)
- [Install](#install)
- [Usage](#usage)
- [Commands](#commands)
- [Flags](#flags)
- [Tools](#tools)
- [How it works](#how-it-works)
- [State](#state)
- [Tailscale](#tailscale)
- [Configuration](#configuration)
- [Development](#development)
- [Troubleshooting](#troubleshooting)

## Prerequisites

- [LXD](https://canonical.com/lxd), installed and initialised (`lxd init --auto`
  is enough to get started).
- Go 1.26+ if building from source.
- Optional host directories that tools bind-mount into instances — missing
  ones are simply skipped:
  - `~/scripts` — a [probuntu](https://github.com/jnsgruk/dotfiles)-style
    dotfiles repo (the `base` tool sources `probuntu/provision-headless`
    from it).
  - `~/.config/gh`, `~/.local/share/atuin` — shared shell history and GitHub CLI state.
  - `~/.codex/auth.json`, `~/.codex/config.toml` — default Codex CLI auth mount and config copy.
  - `~/.config/opencode`, `~/.local/share/opencode/auth.json` — default opencode mounts.
  - `~/.claude`, `~/.claude.json` — when using `--tools claude`.
- [1Password CLI (`op`)](https://developer.1password.com/docs/cli/), signed in,
  if you use `--tailscale` (OAuth client secret and API token are fetched from 1Password).

## Install

```
go install github.com/jnsgruk/dbx/cmd/dbx@latest
```

Or from a checkout:

```
go build -o dbx ./cmd/dbx
```

## Usage

Run `dbx` in a project directory. The first time it creates an instance and
drops you into a shell; subsequent runs in the same directory reconnect to
the same instance.

```bash
cd ~/src/my-app
dbx                    # create or reconnect, open shell
dbx exec -- go test ./...
dbx ls                 # list tracked instances
dbx stop               # stop (keeps state)
dbx rm                 # delete
```

The project directory is mounted at `~/<dirname>` inside the instance.

## Commands

```
dbx [flags]                    Create (or reconnect to) an instance for $PWD and open a shell
dbx create <name>              Create a named instance without opening a shell
dbx shell [name]               Open a shell; resolves from $PWD if [name] is omitted
dbx exec [name] -- <cmd...>    Run a command inside an instance; everything after -- is passed through
dbx ls                         List tracked instances (alias: list)
dbx stop [name]                Stop an instance
dbx rm   [name]                Force-delete an instance and its Tailscale device (alias: delete)
dbx base ls                    List cached base instances
dbx base rm                    Remove a base instance (forces rebuild next time that image is used)
dbx tools list                 List user-selectable tools
```

All subcommands accept `--log-level debug|info|warn|error` (default `info`).

## Flags

Creation flags (apply to the root command and `dbx create`):

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--vm` | | `false` | Create a VM instead of a container |
| `--image` | `-i` | `ubuntu:resolute` | LXD image |
| `--cpu` | `-c` | *(unset; `16` for VMs)* | CPU limit |
| `--mem` | `-m` | *(unset; `16GiB` for VMs)* | Memory limit |
| `--disk` | `-d` | *(unset; `100GiB` for VMs)* | Disk size |
| `--tools` | `-t` | `codex,opencode` | Additional comma-separated tools |
| `--tailscale` | | `false` | Enrol the instance in your tailnet |
| `--name` | `-n` | *(generated)* | Exact instance hostname |
| `--new` | | `false` | Create a new instance even if one exists for this directory |

VM defaults only apply when the corresponding flag isn't explicitly set.
Generated names look like `<dirname>-<image-release>-<4 hex>`.

## Tools

Tools are the unit of provisioning. Each declares its mounts and an install
script; `dbx` composes them into the instance. Some are applied
unconditionally (the core `base` provisioning, Tailscale package install,
VM swapfile). Codex and opencode are included by default; `--tools` adds
more user-selectable tools.

Run `dbx tools list` for the current set. At time of writing:

| Tool | What it does |
|------|--------------|
| `claude` | Installs `claude-code` via mise; mounts `~/.claude` and `~/.claude.json` |
| `codex` | Installs Codex CLI via mise; mounts `~/.codex/auth.json` and copies `~/.codex/config.toml` into the instance (mode 600) |
| `opencode` | Installs `opencode` via mise; mounts config dir and `auth.json` (file mount, mode 600) |
| `k8s` | Installs Canonical k8s snap, bootstraps a single-node cluster with MetalLB, deploys a local registry, writes `~/.kube/config` |
| `nix` | Installs Nix via the Determinate Systems installer |

Example:

```
dbx --tools k8s
```

Adding a tool is a small Go file in `internal/tools/`; see [CLAUDE.md](CLAUDE.md).

## How it works

1. **User remap.** The cloud-init `ubuntu` user is moved to `config.RemapUID`,
   freeing the host UID for the `dbx` user created inside. ID mapping is
   configured so mounted files appear owned by you on both sides.
2. **Base instances.** For each `(image, container|vm)` pair, `dbx` builds a
   base instance in a dedicated LXD project (`dbx`), applies core
   provisioning, stops it, and keeps it. New instances are `lxc copy`'d
   from the base — usually a few seconds end-to-end. Manage with
   `dbx base ls` / `dbx base rm`.
3. **Mounts.** The project directory is mounted at `~/<basename>`. Other
   mounts come from selected tools. Directories use bind mounts; single
   files are bind-mounted on containers unless the tool needs a local copy,
   and `lxc file push`'d on VMs (which can't bind-mount files).
4. **Install pipeline.** Each selected tool's install script runs as the
   target user, wrapped with `#!/usr/bin/env bash` + `set -exo pipefail`.
5. **Readiness gates.** Creation waits, with bounded timeouts, for the
   user to exist, DNS to resolve, `snapd` to be responsive, and an
   interactive shell to succeed.

## State

`dbx` keeps a JSON state file at `~/.local/share/dbx/state.json` mapping
host directories to instance names. This is what lets `dbx` in a known
directory reconnect to the right instance, and `dbx ls` show the
directory each instance came from. On every invocation, entries for
instances no longer present in LXD are pruned.

## Tailscale

Passing `--tailscale` on create:

1. Uses `op` to fetch a Tailscale OAuth client secret and API token from 1Password.
2. Mints an ephemeral, pre-authorised auth key via the OAuth client.
3. Runs `tailscale up --auth-key=... --ssh --operator=<user>` inside the
   instance and records the device ID against the instance in state.

`dbx rm` removes the matching Tailscale device. Other commands prune
devices for instances that have vanished from LXD.

Without `--tailscale`, the `tailscale` package is still installed in
every instance (cheap), but `tailscale up` is never called.

## Configuration

Personalisation lives in compile-time constants in
`internal/config/config.go`:

```go
User        = "jon"              // username created inside instances
GitHubUser  = "jnsgruk"          // ssh-import-id source
Image       = "ubuntu:resolute"  // default image
Shell       = "fish"             // interactive shell
LoginShell  = "/usr/bin/bash"    // /etc/passwd login shell
RemapUID    = "1100"             // where the cloud-init `ubuntu` user is moved
```

Fork and edit these. There is no runtime config file.

## Development

```bash
mise install          # one-time: install pinned dev tools
prek install          # one-time: install git hooks

mise run fmt          # gofumpt
mise run lint         # go vet
mise run build        # go build -o dbx ./cmd/dbx
mise run check        # fmt + lint + test + build

prek run -av          # run all git hooks over the tree
go test ./...
```

Commits follow [Conventional Commits](https://www.conventionalcommits.org/).
Architectural notes and conventions live in [CLAUDE.md](CLAUDE.md).

## Troubleshooting

- **`directory has multiple instances: ...`** — more than one instance is
  registered for this directory. Use `-n <name>` to pick one, or `--new`
  to create another.
- **`timed out waiting for user/network/snapd in instance`** — the
  instance didn't come up cleanly. Inspect with `lxc info <name>` /
  `lxc console <name>`; re-run with `-l debug` for the full lxc command log.
- **Stale base instance.** After changing base provisioning: `dbx base rm`
  (with `--vm` / `--image` if relevant).
- **File ownership looks wrong on mounts.** ID mapping assumes `id -u`
  of the user running `dbx` matches the host UID used for the new user.
