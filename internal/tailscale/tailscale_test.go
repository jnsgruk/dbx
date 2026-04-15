package tailscale

import (
	"os"
	"path/filepath"
	"testing"
)

func withTempStatePath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	orig := statePath
	statePath = func() string {
		return filepath.Join(dir, "tailscale.json")
	}
	t.Cleanup(func() { statePath = orig })
	return dir
}

func TestLoadDevicesMissingFile(t *testing.T) {
	withTempStatePath(t)
	devices := LoadDevices()
	if len(devices) != 0 {
		t.Fatalf("expected empty map, got %v", devices)
	}
}

func TestSaveAndLoadDevices(t *testing.T) {
	withTempStatePath(t)

	if err := SaveDevice("instance-a", "device-123"); err != nil {
		t.Fatalf("SaveDevice returned error: %v", err)
	}
	if err := SaveDevice("instance-b", "device-456"); err != nil {
		t.Fatalf("SaveDevice returned error: %v", err)
	}

	devices := LoadDevices()
	if len(devices) != 2 {
		t.Fatalf("expected 2 devices, got %d: %v", len(devices), devices)
	}
	if devices["instance-a"] != "device-123" {
		t.Errorf("devices[instance-a] = %q, want %q", devices["instance-a"], "device-123")
	}
	if devices["instance-b"] != "device-456" {
		t.Errorf("devices[instance-b] = %q, want %q", devices["instance-b"], "device-456")
	}
}

func TestLoadDevicesCorruptJSON(t *testing.T) {
	dir := withTempStatePath(t)
	if err := os.WriteFile(filepath.Join(dir, "tailscale.json"), []byte("not json"), 0o644); err != nil {
		t.Fatalf("writing corrupt file: %v", err)
	}

	devices := LoadDevices()
	if len(devices) != 0 {
		t.Fatalf("expected empty map for corrupt JSON, got %v", devices)
	}
}
