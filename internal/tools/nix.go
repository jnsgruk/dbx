package tools

import "strings"

type nix struct{}

func (nix) Name() string           { return "nix" }
func (nix) Description() string    { return "Nix (Determinate Systems installer)" }
func (nix) Mounts(Context) []Mount { return nil }

func (nix) InstallScript(Context) string {
	return strings.Join([]string{
		`sudo mkdir -p "/nix/var/nix/profiles/per-user/${USER}"`,
		`curl -sSfL https://install.determinate.systems/nix | sh -s -- install --no-confirm`,
	}, "\n")
}

func init() { Register(nix{}) }
