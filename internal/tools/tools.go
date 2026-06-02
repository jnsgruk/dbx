// Package tools models the installable/mountable tools applied to a dbx
// instance. Each Tool contributes mounts and/or an install script. Some tools
// are Always selected (the base provisioning, VM setup, Tailscale). Others
// are selected by CLI defaults or via --tools.
package tools

import (
	"fmt"
	"slices"
	"sort"
)

// Mount describes a host path exposed inside the instance.
type Mount struct {
	Name   string
	Source string
	Dest   string
	File   bool   // true for single files (VMs copy these instead of mounting)
	Copy   bool   // true for single files copied instead of mounted
	Mode   string // file permission mode for copied files, e.g. "600"
	Build  bool   // true if the mount is needed during base image build
}

// Context carries the information a tool needs to compute its mounts and
// install script. All paths are absolute.
type Context struct {
	HostHome     string
	InstanceHome string
	Cwd          string
	User         string
	GitHubUser   string
	VM           bool
	Project      string // LXD project ("" for default)
	Instance     string // instance name
}

// Tool is an installable/mountable capability applied to an instance.
type Tool interface {
	Name() string
	Description() string

	// Mounts returns host->instance mounts this tool requires. May be empty.
	Mounts(ctx Context) []Mount

	// InstallScript returns the bash body run as the target user. The caller
	// wraps it with shebang + set -ex. Return "" for no install step.
	InstallScript(ctx Context) string
}

// Always is implemented by tools that should be selected unconditionally for
// the given context (e.g. base, vm when opts.VM, tailscale).
type Always interface {
	Always(ctx Context) bool
}

// Hidden is implemented by tools that should not appear in user-facing
// listings (help text, `dbx tools list`). Typically implementation details
// like base provisioning or VM swap setup.
type Hidden interface {
	Hidden() bool
}

// IsHidden reports whether a tool is hidden from user-facing listings.
func IsHidden(t Tool) bool {
	if h, ok := t.(Hidden); ok {
		return h.Hidden()
	}
	return false
}

// Authenticator is implemented by tools that need post-install authentication
// using a runtime-supplied secret. The returned string is an opaque identifier
// (e.g. Tailscale device ID) that the caller may persist.
type Authenticator interface {
	Authenticate(ctx Context, key string) (string, error)
}

var registry []Tool

// Register adds a tool to the global registry. Intended to be called from
// init(). Order of registration determines the order returned by All().
func Register(t Tool) {
	for _, existing := range registry {
		if existing.Name() == t.Name() {
			panic(fmt.Sprintf("tool %q already registered", t.Name()))
		}
	}
	registry = append(registry, t)
}

// All returns every registered tool in registration order.
func All() []Tool {
	out := make([]Tool, len(registry))
	copy(out, registry)
	return out
}

// Get returns a tool by name, or (nil, false) if not found.
func Get(name string) (Tool, bool) {
	for _, t := range registry {
		if t.Name() == name {
			return t, true
		}
	}
	return nil, false
}

// Names returns user-selectable tool names sorted alphabetically. Hidden
// tools (always-on plumbing) are excluded. Useful for help text and
// validation errors.
func Names() []string {
	names := make([]string, 0, len(registry))
	for _, t := range registry {
		if IsHidden(t) {
			continue
		}
		names = append(names, t.Name())
	}
	sort.Strings(names)
	return names
}

// Validate ensures each requested name matches a registered, user-selectable
// tool. Hidden tools cannot be selected directly.
func Validate(requested []string) error {
	available := Names()
	for _, name := range requested {
		if !slices.Contains(available, name) {
			return fmt.Errorf("unknown tool %q (available: %v)", name, available)
		}
	}
	return nil
}

// Select returns the tools applicable to the instance: all Always tools for
// the context plus any user-requested tools. Order matches registration.
// User-requested tools that are also Always are deduplicated.
func Select(ctx Context, requested []string) []Tool {
	var out []Tool
	for _, t := range registry {
		if a, ok := t.(Always); ok && a.Always(ctx) {
			out = append(out, t)
			continue
		}
		if slices.Contains(requested, t.Name()) {
			out = append(out, t)
		}
	}
	return out
}

// SelectAlways returns only the Always tools for the given context, in
// registration order. Used during base image build.
func SelectAlways(ctx Context) []Tool {
	var out []Tool
	for _, t := range registry {
		if a, ok := t.(Always); ok && a.Always(ctx) {
			out = append(out, t)
		}
	}
	return out
}
