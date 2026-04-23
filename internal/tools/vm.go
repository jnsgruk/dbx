package tools

import "strings"

// vm adds an 8G swapfile. Always selected when Context.VM is true.
type vm struct{}

func (vm) Name() string            { return "vm" }
func (vm) Description() string     { return "VM-specific setup (swapfile)" }
func (vm) Always(ctx Context) bool { return ctx.VM }
func (vm) Hidden() bool            { return true }
func (vm) Mounts(Context) []Mount  { return nil }

func (vm) InstallScript(Context) string {
	return strings.Join([]string{
		"sudo fallocate -l 8G /swapfile",
		"sudo chmod 600 /swapfile",
		"sudo mkswap /swapfile",
		"sudo swapon /swapfile",
		`printf '/swapfile none swap sw 0 0\n' | sudo tee -a /etc/fstab`,
	}, "\n")
}

func init() { Register(vm{}) }
