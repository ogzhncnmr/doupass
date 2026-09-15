package codex

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"

	"github.com/ogzhncnmr/doupass/internal/fsutil"
)

const marker = "doupass"

type Result struct {
	Changed bool
	Servers int
	Backup  string
	Path    string
}

// WrapServers routes every local MCP server in a Codex config.toml through the
// doupass proxy. Servers without a command (remote URL servers) are skipped,
// and already-wrapped servers make the operation a no-op.
func WrapServers(path string) (Result, error) {
	return mutate(path, true)
}

// UnwrapServers restores the original commands of doupass-wrapped MCP servers.
func UnwrapServers(path string) (Result, error) {
	return mutate(path, false)
}

// HasWrappers reports whether any MCP server in the config is doupass-wrapped.
func HasWrappers(path string) bool {
	doc, err := loadDoc(path)
	if err != nil {
		return false
	}
	servers, _ := doc["mcp_servers"].(map[string]any)
	for _, raw := range servers {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if cmd, _ := entry["command"].(string); cmd == marker {
			return true
		}
	}
	return false
}

func mutate(path string, wrap bool) (Result, error) {
	res := Result{Path: path}
	//#nosec G304 -- path is the Codex config.toml selected by the local user
	data, err := os.ReadFile(path)
	if err != nil {
		return res, err
	}
	var doc map[string]any
	if err := toml.Unmarshal(data, &doc); err != nil {
		return res, fmt.Errorf("parse %s: %w", path, err)
	}
	servers, _ := doc["mcp_servers"].(map[string]any)
	changed := 0
	for name, raw := range servers {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		cmd, _ := entry["command"].(string)
		if wrap {
			if cmd == "" || cmd == marker {
				continue
			}
			args := toStrSlice(entry["args"])
			newArgs := make([]any, 0, len(args)+4)
			newArgs = append(newArgs, "proxy", "--server", name, "--", cmd)
			for _, a := range args {
				newArgs = append(newArgs, a)
			}
			entry["command"] = marker
			entry["args"] = newArgs
			changed++
		} else {
			if cmd != marker {
				continue
			}
			args := toStrSlice(entry["args"])
			idx := -1
			for i, a := range args {
				if a == "--" {
					idx = i
					break
				}
			}
			if idx < 0 || idx+1 >= len(args) {
				continue
			}
			entry["command"] = args[idx+1]
			entry["args"] = args[idx+2:]
			changed++
		}
	}
	if changed == 0 {
		return res, nil
	}
	backup := path + ".doupass.bak"
	mode := os.FileMode(0o600)
	if fi, statErr := os.Stat(path); statErr == nil {
		mode = fi.Mode()
	}
	//#nosec G703 -- backup path derives from the user-selected config path
	if err := os.WriteFile(backup, data, mode); err != nil {
		return res, fmt.Errorf("write backup: %w", err)
	}
	out, err := toml.Marshal(doc)
	if err != nil {
		return res, err
	}
	if err := fsutil.WriteFileAtomic(path, out, mode); err != nil {
		return res, err
	}
	return Result{Changed: true, Servers: changed, Backup: backup, Path: path}, nil
}

func loadDoc(path string) (map[string]any, error) {
	//#nosec G304 -- path is the Codex config.toml selected by the local user
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := toml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func toStrSlice(v any) []string {
	raw, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
