package tailscale

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	tsclient "github.com/tailscale/tailscale-client-go/v2"
)

const (
	opItem  = "Tailscale OAuth"
	opVault = "Devbox"
)

// AuthKey fetches Tailscale OAuth credentials from 1Password and generates
// a non-ephemeral, single-use auth key tagged with "devbox".
func AuthKey() (string, error) {
	client, err := newClient()
	if err != nil {
		return "", err
	}

	key, err := client.Keys().Create(context.Background(), tsclient.CreateKeyRequest{
		Capabilities: tsclient.KeyCapabilities{
			Devices: struct {
				Create struct {
					Reusable      bool     `json:"reusable"`
					Ephemeral     bool     `json:"ephemeral"`
					Tags          []string `json:"tags"`
					Preauthorized bool     `json:"preauthorized"`
				} `json:"create"`
			}{
				Create: struct {
					Reusable      bool     `json:"reusable"`
					Ephemeral     bool     `json:"ephemeral"`
					Tags          []string `json:"tags"`
					Preauthorized bool     `json:"preauthorized"`
				}{
					Reusable:      false,
					Ephemeral:     false,
					Preauthorized: true,
					Tags:          []string{"tag:devbox"},
				},
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("generating tailscale auth key: %w", err)
	}
	return key.Key, nil
}

// RemoveDeviceByInstance looks up the device ID for the given instance,
// removes it from the tailnet, and deletes the state entry.
func RemoveDeviceByInstance(instanceName string) error {
	devices := LoadDevices()
	deviceID, ok := devices[instanceName]
	if !ok {
		return nil
	}

	client, err := newClient()
	if err != nil {
		return err
	}

	if err := client.Devices().Delete(context.Background(), deviceID); err != nil {
		return fmt.Errorf("removing tailscale device: %w", err)
	}

	delete(devices, instanceName)
	return saveDevices(devices)
}

// PruneDevices removes tailnet devices for instances that no longer exist.
func PruneDevices(liveInstances map[string]struct{}) {
	devices := LoadDevices()
	if len(devices) == 0 {
		return
	}

	var stale []string
	for name := range devices {
		if _, ok := liveInstances[name]; !ok {
			stale = append(stale, name)
		}
	}
	if len(stale) == 0 {
		return
	}

	client, err := newClient()
	if err != nil {
		slog.Warn("Pruning Tailscale devices", "error", err)
		return
	}

	changed := false
	for _, name := range stale {
		deviceID := devices[name]
		slog.Info("Removing stale Tailscale device", "instance", name, "device", deviceID)
		if err := client.Devices().Delete(context.Background(), deviceID); err != nil {
			slog.Warn("Removing stale Tailscale device", "instance", name, "error", err)
			continue
		}
		delete(devices, name)
		changed = true
	}
	if changed {
		if err := saveDevices(devices); err != nil {
			slog.Warn("Saving Tailscale device state", "error", err)
		}
	}
}

// SaveDevice persists the mapping from instance name to tailscale device ID.
func SaveDevice(instanceName, deviceID string) error {
	devices := LoadDevices()
	devices[instanceName] = deviceID
	return saveDevices(devices)
}

// LoadDevices loads the instance→device mapping from disk.
func LoadDevices() map[string]string {
	data, err := os.ReadFile(statePath())
	if err != nil {
		return map[string]string{}
	}
	var devices map[string]string
	if err := json.Unmarshal(data, &devices); err != nil {
		slog.Warn("Loading Tailscale device state, starting fresh", "error", err)
		return map[string]string{}
	}
	return devices
}

var statePath = func() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "dbx", "tailscale.json")
}

func saveDevices(devices map[string]string) error {
	p := statePath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(devices, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}

func newClient() (*tsclient.Client, error) {
	clientID, clientSecret, err := fetchOAuthCredentials()
	if err != nil {
		return nil, err
	}

	oauthCfg := tsclient.OAuthConfig{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       []string{"all:write"},
	}

	return &tsclient.Client{
		Tailnet: "-",
		HTTP:    oauthCfg.HTTPClient(),
	}, nil
}

type opField struct {
	Value string `json:"value"`
}

func fetchOAuthCredentials() (clientID, clientSecret string, err error) {
	cmd := exec.Command(
		"op", fmt.Sprintf("--vault=%s", opVault), "item", "get", opItem,
		"--fields", "label=client_id,label=client_secret",
		"--format", "json",
	)
	output, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("fetching tailscale credentials from 1password: %w", err)
	}

	var fields []opField
	if err := json.Unmarshal(output, &fields); err != nil {
		return "", "", fmt.Errorf("parsing 1password response: %w", err)
	}
	if len(fields) < 2 {
		return "", "", fmt.Errorf("fetching tailscale credentials from 1password: expected 2 fields, got %d", len(fields))
	}
	return fields[0].Value, fields[1].Value, nil
}
