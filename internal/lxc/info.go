package lxc

import (
	"bytes"
	"encoding/csv"
	"strings"
	"time"
)

type InstanceInfo struct {
	Status    string
	IPv4      string
	CreatedAt time.Time
}

// ListInstances returns the names of all LXD instances in a single call.
func ListInstances() (map[string]struct{}, error) {
	output, err := Run("list", "--format=csv", "-c", "n")
	if err != nil {
		return nil, err
	}
	names := make(map[string]struct{})
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line != "" {
			names[line] = struct{}{}
		}
	}
	return names, nil
}

// ListInstanceInfo returns status, IPv4, and creation time for all instances in a single call.
func ListInstanceInfo() (map[string]InstanceInfo, error) {
	output, err := Run("list", "--format=csv", "-c", "ns4c")
	if err != nil {
		return nil, err
	}
	result := make(map[string]InstanceInfo)
	r := csv.NewReader(bytes.NewReader(output))
	for {
		record, err := r.Read()
		if err != nil {
			break
		}
		if len(record) < 2 {
			continue
		}
		info := InstanceInfo{Status: record[1]}
		if len(record) > 2 {
			info.IPv4 = parseFirstRoutableIPv4(record[2])
		}
		if len(record) > 3 {
			info.CreatedAt, _ = time.Parse("2006/01/02 15:04 MST", record[3])
		}
		result[record[0]] = info
	}
	return result, nil
}

func GetInstanceInfo(name string) InstanceInfo {
	output, err := Run("list", name, "--format=csv", "-c", "s4")
	if err != nil {
		return InstanceInfo{}
	}

	r := csv.NewReader(bytes.NewReader(output))
	record, err := r.Read()
	if err != nil || len(record) < 1 {
		return InstanceInfo{}
	}

	info := InstanceInfo{Status: record[0]}
	if len(record) > 1 {
		info.IPv4 = parseFirstRoutableIPv4(record[1])
	}
	return info
}

// parseFirstRoutableIPv4 extracts the first IPv4 address that isn't on a
// known virtual interface (docker, lxdbr, cilium, veth, flannel, cni, br-).
// The lxc list output for IPv4 looks like: "10.0.0.1 (eth0)\n172.17.0.1 (docker0)".
func parseFirstRoutableIPv4(raw string) string {
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		ip, iface, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		iface = strings.Trim(iface, "()")
		if isVirtualInterface(iface) {
			continue
		}
		return ip
	}
	return ""
}

func isVirtualInterface(name string) bool {
	for _, prefix := range []string{"docker", "lxdbr", "cilium", "veth", "flannel", "cni", "br-", "virbr"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}
