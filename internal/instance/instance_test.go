package instance

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/jnsgruk/dbx/internal/tools"
)

func TestImageBase(t *testing.T) {
	tests := []struct {
		name  string
		image string
		want  string
	}{
		{name: "standard image", image: "ubuntu:resolute", want: "resolute"},
		{name: "no colon", image: "myimage", want: "myimage"},
		{name: "multiple colons", image: "registry:port:tag", want: "tag"},
		{name: "trailing colon", image: "ubuntu:", want: ""},
		{name: "colon at start", image: ":foo", want: "foo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := imageBase(tt.image)
			if got != tt.want {
				t.Errorf("imageBase(%q) = %q, want %q", tt.image, got, tt.want)
			}
		})
	}
}

func TestBaseName(t *testing.T) {
	tests := []struct {
		name  string
		image string
		vm    bool
		want  string
	}{
		{name: "container", image: "ubuntu:resolute", vm: false, want: "base-resolute-container"},
		{name: "vm", image: "ubuntu:resolute", vm: true, want: "base-resolute-vm"},
		{name: "different image", image: "ubuntu:noble", vm: false, want: "base-noble-container"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BaseName(tt.image, tt.vm)
			if got != tt.want {
				t.Errorf("BaseName(%q, %v) = %q, want %q", tt.image, tt.vm, got, tt.want)
			}
		})
	}
}

func TestGenerateName(t *testing.T) {
	pattern := regexp.MustCompile(`^myproject-resolute-[0-9a-f]{4}$`)

	name, err := GenerateName("myproject", "ubuntu:resolute")
	if err != nil {
		t.Fatalf("GenerateName returned error: %v", err)
	}
	if !pattern.MatchString(name) {
		t.Errorf("GenerateName() = %q, does not match pattern %s", name, pattern)
	}

	name2, err := GenerateName("myproject", "ubuntu:resolute")
	if err != nil {
		t.Fatalf("GenerateName returned error: %v", err)
	}
	if name == name2 {
		t.Errorf("two GenerateName calls produced identical names: %q", name)
	}
}

func TestPrepareMountSourceTransformsSecureCopy(t *testing.T) {
	hostSource := filepath.Join(t.TempDir(), "config.toml")
	hostContents := []byte("secret = true\n")
	if err := os.WriteFile(hostSource, hostContents, 0o644); err != nil {
		t.Fatal(err)
	}

	prepared, cleanup, err := prepareMountSource(tools.Mount{
		Source: hostSource,
		Transform: func(input []byte) ([]byte, error) {
			return append(input, []byte("enabled = false\n")...), nil
		},
	})
	if err != nil {
		t.Fatalf("prepareMountSource() returned error: %v", err)
	}
	t.Cleanup(cleanup)

	info, err := os.Stat(prepared)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("temporary mode = %o, want 600", got)
	}
	gotHost, err := os.ReadFile(hostSource)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotHost) != string(hostContents) {
		t.Errorf("host source changed to %q", gotHost)
	}
	gotPrepared, err := os.ReadFile(prepared)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotPrepared) != "secret = true\nenabled = false\n" {
		t.Errorf("prepared source = %q", gotPrepared)
	}

	cleanup()
	if _, err := os.Stat(prepared); !os.IsNotExist(err) {
		t.Errorf("temporary file still exists after cleanup: %v", err)
	}
}

func TestPollUntil(t *testing.T) {
	t.Run("immediately true", func(t *testing.T) {
		ok := pollUntil(time.Second, func() bool { return true })
		if !ok {
			t.Error("pollUntil returned false when check immediately returns true")
		}
	})

	t.Run("becomes true after several calls", func(t *testing.T) {
		calls := 0
		ok := pollUntil(5*time.Second, func() bool {
			calls++
			return calls >= 3
		})
		if !ok {
			t.Error("pollUntil returned false when check should have succeeded")
		}
		if calls < 3 {
			t.Errorf("expected at least 3 calls, got %d", calls)
		}
	})

	t.Run("timeout reached", func(t *testing.T) {
		ok := pollUntil(50*time.Millisecond, func() bool { return false })
		if ok {
			t.Error("pollUntil returned true when check always returns false")
		}
	})
}

func TestShellReadyArgs(t *testing.T) {
	args := shellReadyArgs("jon")
	if len(args) == 0 {
		t.Fatal("shellReadyArgs() returned no args for fish shell")
	}

	want := []string{
		"sudo", "-u", "jon", "-i", "bash", "-lc",
		"command -v mise >/dev/null && mise --version >/dev/null",
	}

	if len(args) != len(want) {
		t.Fatalf("shellReadyArgs() = %v, want %v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("shellReadyArgs() = %v, want %v", args, want)
		}
	}
}
