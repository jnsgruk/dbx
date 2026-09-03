package tools

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

type codex struct{}

func (codex) Name() string { return "codex" }
func (codex) Description() string {
	return "Codex CLI (via pnpm) with config/auth mounts"
}

func (codex) Mounts(ctx Context) []Mount {
	return []Mount{
		{
			Name:   "codexauth",
			Source: filepath.Join(ctx.HostHome, ".codex", "auth.json"),
			Dest:   filepath.Join(ctx.InstanceHome, ".codex", "auth.json"),
			File:   true,
			Mode:   "600",
		},
		{
			Name:      "codexconfig",
			Source:    filepath.Join(ctx.HostHome, ".codex", "config.toml"),
			Dest:      filepath.Join(ctx.InstanceHome, ".codex", "config.toml"),
			File:      true,
			Copy:      true,
			Mode:      "600",
			Transform: disableNodeREPL,
		},
	}
}

var (
	tableHeaderPattern   = regexp.MustCompile(`^[ \t]*\[\[?[^\[\]\r\n]+\]\]?[ \t]*(?:#.*)?$`)
	nodeREPLTablePattern = regexp.MustCompile(`^[ \t]*\[[ \t]*mcp_servers[ \t]*\.[ \t]*node_repl[ \t]*\][ \t]*(?:#.*)?$`)
	enabledPattern       = regexp.MustCompile(`^([ \t]*enabled[ \t]*=[ \t]*)([^#]*?)([ \t]*(?:#.*)?)$`)
)

type configLine struct {
	body string
	eol  string
}

// disableNodeREPL preserves the source TOML text while forcing the direct
// enabled key in the exact mcp_servers.node_repl table to false.
func disableNodeREPL(input []byte) ([]byte, error) {
	lines := splitConfigLines(input)
	var output []configLine
	inNodeREPL := false
	foundEnabled := false
	sectionEOL := "\n"
	changed := false

	insertEnabled := func(atEOF bool) {
		line := configLine{body: "enabled = false", eol: sectionEOL}
		insertAt := len(output)
		for insertAt > 0 && strings.HasPrefix(strings.TrimSpace(output[insertAt-1].body), "#") {
			insertAt--
		}
		if insertAt < len(output) {
			output = append(output, configLine{})
			copy(output[insertAt+1:], output[insertAt:])
			output[insertAt] = line
		} else if atEOF && len(output) > 0 && output[len(output)-1].eol == "" {
			output[len(output)-1].eol = sectionEOL
			line.eol = ""
			output = append(output, line)
		} else {
			output = append(output, line)
		}
		changed = true
	}

	for _, line := range lines {
		if tableHeaderPattern.MatchString(line.body) {
			if inNodeREPL && !foundEnabled {
				insertEnabled(false)
			}
			inNodeREPL = nodeREPLTablePattern.MatchString(line.body)
			foundEnabled = false
			if inNodeREPL && line.eol != "" {
				sectionEOL = line.eol
			}
			output = append(output, line)
			continue
		}

		if inNodeREPL {
			match := enabledPattern.FindStringSubmatch(line.body)
			if match != nil {
				foundEnabled = true
				switch strings.TrimSpace(match[2]) {
				case "true":
					line.body = match[1] + "false" + match[3]
					changed = true
				case "false":
				default:
					return nil, fmt.Errorf("parsing node_repl enabled setting: expected boolean")
				}
			}
		}
		output = append(output, line)
	}
	if inNodeREPL && !foundEnabled {
		insertEnabled(true)
	}
	if !changed {
		return input, nil
	}

	var result bytes.Buffer
	for _, line := range output {
		result.WriteString(line.body)
		result.WriteString(line.eol)
	}
	return result.Bytes(), nil
}

func splitConfigLines(input []byte) []configLine {
	if len(input) == 0 {
		return nil
	}
	parts := bytes.SplitAfter(input, []byte("\n"))
	lines := make([]configLine, 0, len(parts))
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		line := configLine{body: string(part)}
		switch {
		case bytes.HasSuffix(part, []byte("\r\n")):
			line.body = string(part[:len(part)-2])
			line.eol = "\r\n"
		case bytes.HasSuffix(part, []byte("\n")):
			line.body = string(part[:len(part)-1])
			line.eol = "\n"
		}
		lines = append(lines, line)
	}
	return lines
}

func (codex) InstallScript(Context) string {
	return `command -v pnpm >/dev/null || mise use -g pnpm@latest
mise use -g nodejs
mkdir -p ~/.local/bin
pnpm config set global-bin-dir ~/.local/bin
pnpm add --global @openai/codex`
}

func init() { Register(codex{}) }
