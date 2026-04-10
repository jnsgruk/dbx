# dbx

LXD development instance creator. Creates and manages containers and VMs with standard mounts and provisioning.

`dbx` gives you disposable, pre-provisioned Ubuntu development environments backed by LXD. Run `dbx` in any project directory to get a shell inside a fresh container (or VM) with your project mounted, tools installed, and dotfiles ready.

## Prerequisites

- [LXD](https://documentation.ubuntu.com/lxd/en/latest/) installed and initialised
- Go 1.26+ (to build from source)

## Install

```
go install github.com/jnsgruk/dbx/cmd/dbx@latest
```

Or build locally:

```
go build -o dbx ./cmd/dbx
```

## Quick start

```bash
# Create an instance for the current directory and drop into a shell
dbx

# Next time you run dbx in the same directory, it reconnects to the same instance
dbx
```

That's it. The current directory is mounted at `~/project` inside the instance.

## Usage

```
dbx [flags]            # Create (or reconnect to) an instance for the current directory
dbx create <name>      # Create a named instance
dbx shell [name]       # Open a shell (resolves from cwd if name omitted)
dbx ls                 # List tracked instances
dbx stop [name]        # Stop an instance
dbx rm [name]          # Force-delete an instance
dbx base ls            # List base instances used for fast creation
dbx base rm            # Remove a base instance
```

### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--vm` | | `false` | Create a virtual machine instead of a container |
| `--image` | `-i` | `ubuntu:questing` | Base image |
| `--cpu` | `-c` | | CPU limit (e.g. `4`) |
| `--mem` | `-m` | | Memory limit (e.g. `8GiB`) |
| `--disk` | `-d` | | Disk size (e.g. `50GiB`) |
| `--extras` | `-e` | | Comma-separated extras (see below) |
| `--tailscale` | | `false` | Install and authenticate Tailscale |
| `--log-level` | `-l` | `info` | Log verbosity: `debug`, `info`, `warn`, `error` |

When `--vm` is used, defaults are applied for CPU (16), memory (16GiB), and disk (100GiB) unless explicitly overridden.

## Extras

Optional provisioning scripts that run after instance creation:

- **claude** -- Claude Code setup
- **k8s** -- Kubernetes tooling
- **nix** -- Nix package manager (via Determinate Systems installer)

```bash
dbx --extras k8s,claude
```

## How it works

1. **User creation** -- `dbx` creates a user inside the instance matching your host username and UID, with passwordless sudo. The cloud-init `ubuntu` user is remapped out of the way.
2. **Base instances** -- `dbx` builds a base instance with standard provisioning (shell config, SSH keys, common packages) and caches it in a separate LXD project. Subsequent creates copy from this base for fast startup.
3. **Mounts** -- The current directory, scripts, and config directories (`~/.claude`, `~/.config/gh`, `~/.local/share/atuin`) are bind-mounted into the instance. Container ID mapping ensures host UID ownership is preserved.
4. **State tracking** -- A JSON state file (`~/.local/share/dbx/state.json`) maps working directories to instance names, so `dbx` can reconnect you to the right instance automatically.
