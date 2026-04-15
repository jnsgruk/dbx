package lxc

import "testing"

func TestProjectArgs(t *testing.T) {
	tests := []struct {
		name    string
		project string
		args    []string
		want    []string
	}{
		{
			name:    "empty project returns args unchanged",
			project: "",
			args:    []string{"info", "myinstance"},
			want:    []string{"info", "myinstance"},
		},
		{
			name:    "empty args returns empty",
			project: "dbx",
			args:    []string{},
			want:    []string{},
		},
		{
			name:    "single arg gets project inserted after subcommand",
			project: "dbx",
			args:    []string{"list"},
			want:    []string{"list", "--project", "dbx"},
		},
		{
			name:    "multiple args get project after first",
			project: "dbx",
			args:    []string{"info", "myinstance"},
			want:    []string{"info", "--project", "dbx", "myinstance"},
		},
		{
			name:    "many trailing args preserved",
			project: "myproj",
			args:    []string{"list", "--format", "json", "-c", "ns4"},
			want:    []string{"list", "--project", "myproj", "--format", "json", "-c", "ns4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := projectArgs(tt.project, tt.args...)
			if len(got) != len(tt.want) {
				t.Fatalf("projectArgs(%q, %v) = %v, want %v", tt.project, tt.args, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("projectArgs(%q, %v) = %v, want %v", tt.project, tt.args, got, tt.want)
				}
			}
		})
	}
}

func TestParseFirstRoutableIPv4(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "empty input", raw: "", want: ""},
		{name: "single routable address", raw: "10.0.0.5 (eth0)", want: "10.0.0.5"},
		{name: "docker interface skipped", raw: "172.17.0.1 (docker0)\n10.0.0.5 (eth0)", want: "10.0.0.5"},
		{name: "lxdbr interface skipped", raw: "10.0.0.1 (lxdbr0)\n192.168.1.100 (enp5s0)", want: "192.168.1.100"},
		{name: "all virtual returns empty", raw: "172.17.0.1 (docker0)\n10.0.0.1 (lxdbr0)", want: ""},
		{name: "multiple routable returns first", raw: "10.0.0.5 (eth0)\n192.168.1.100 (enp5s0)", want: "10.0.0.5"},
		{name: "veth skipped", raw: "10.0.0.1 (veth123abc)\n10.0.0.5 (eth0)", want: "10.0.0.5"},
		{name: "flannel skipped", raw: "10.244.0.1 (flannel.1)\n10.0.0.5 (eth0)", want: "10.0.0.5"},
		{name: "cni skipped", raw: "10.0.0.1 (cni0)\n10.0.0.5 (eth0)", want: "10.0.0.5"},
		{name: "br- skipped", raw: "172.18.0.1 (br-abc123)\n10.0.0.5 (eth0)", want: "10.0.0.5"},
		{name: "virbr skipped", raw: "192.168.122.1 (virbr0)\n10.0.0.5 (eth0)", want: "10.0.0.5"},
		{name: "cilium skipped", raw: "10.0.0.1 (cilium_host)\n10.0.0.5 (eth0)", want: "10.0.0.5"},
		{name: "no parenthesised iface skipped", raw: "10.0.0.1\n10.0.0.5 (eth0)", want: "10.0.0.5"},
		{name: "whitespace lines skipped", raw: "  \n\n10.0.0.5 (eth0)", want: "10.0.0.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseFirstRoutableIPv4(tt.raw)
			if got != tt.want {
				t.Errorf("parseFirstRoutableIPv4(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestIsVirtualInterface(t *testing.T) {
	virtual := []string{
		"docker0", "docker1", "lxdbr0", "cilium_host", "cilium_net",
		"veth123abc", "flannel.1", "cni0", "br-abc123", "virbr0",
	}
	for _, name := range virtual {
		t.Run(name+"_is_virtual", func(t *testing.T) {
			if !isVirtualInterface(name) {
				t.Errorf("isVirtualInterface(%q) = false, want true", name)
			}
		})
	}

	physical := []string{"eth0", "enp5s0", "wlan0", "lo", "ens3"}
	for _, name := range physical {
		t.Run(name+"_not_virtual", func(t *testing.T) {
			if isVirtualInterface(name) {
				t.Errorf("isVirtualInterface(%q) = true, want false", name)
			}
		})
	}
}
