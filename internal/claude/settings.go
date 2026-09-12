package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const hookMarker = "doupass hook"

type Result struct {
	Path    string
	Changed bool
	Backup  string
}

func InstallHook(settingsPath, command string) (Result, error) {
	settings, original, err := readSettings(settingsPath)
	if err != nil {
		return Result{Path: settingsPath}, err
	}
	if hasDoupassHook(settings) {
		return Result{Path: settingsPath}, nil
	}
	hooks := ensureMap(settings, "hooks")
	groups := ensureSlice(hooks, "PreToolUse")
	group := map[string]any{
		"hooks": []any{
			map[string]any{"type": "command", "command": command},
		},
	}
	hooks["PreToolUse"] = append(groups, group)
	return writeSettings(settingsPath, settings, original, true)
}

func UninstallHook(settingsPath string) (Result, error) {
	settings, original, err := readSettings(settingsPath)
	if err != nil {
		return Result{Path: settingsPath}, err
	}
	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		return Result{Path: settingsPath}, nil
	}
	groups, ok := hooks["PreToolUse"].([]any)
	if !ok {
		return Result{Path: settingsPath}, nil
	}
	changed := false
	var keptGroups []any
	for _, g := range groups {
		group, ok := g.(map[string]any)
		if !ok {
			keptGroups = append(keptGroups, g)
			continue
		}
		handlers, ok := group["hooks"].([]any)
		if !ok {
			keptGroups = append(keptGroups, g)
			continue
		}
		var keptHandlers []any
		for _, h := range handlers {
			handler, ok := h.(map[string]any)
			if ok {
				if cmd, _ := handler["command"].(string); matchesHook(cmd) {
					changed = true
					continue
				}
			}
			keptHandlers = append(keptHandlers, h)
		}
		if len(keptHandlers) == 0 {
			changed = true
			continue
		}
		group["hooks"] = keptHandlers
		keptGroups = append(keptGroups, group)
	}
	if !changed {
		return Result{Path: settingsPath}, nil
	}
	if len(keptGroups) == 0 {
		delete(hooks, "PreToolUse")
	} else {
		hooks["PreToolUse"] = keptGroups
	}
	if len(hooks) == 0 {
		delete(settings, "hooks")
	}
	return writeSettings(settingsPath, settings, original, true)
}

func readSettings(path string) (map[string]any, []byte, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]any{}, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	settings := map[string]any{}
	if len(raw) == 0 {
		return settings, raw, nil
	}
	if err := json.Unmarshal(raw, &settings); err != nil {
		return nil, nil, fmt.Errorf("claude: %s is not valid JSON: %w", path, err)
	}
	return settings, raw, nil
}

func writeSettings(path string, settings map[string]any, original []byte, changed bool) (Result, error) {
	res := Result{Path: path}
	if !changed {
		return res, nil
	}
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return res, err
	}
	out = append(out, '\n')
	if len(original) > 0 {
		backup := path + ".doupass.bak"
		if err := os.WriteFile(backup, original, 0o600); err == nil {
			res.Backup = backup
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return res, err
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return res, err
	}
	res.Changed = true
	return res, nil
}

func ensureMap(parent map[string]any, key string) map[string]any {
	if m, ok := parent[key].(map[string]any); ok {
		return m
	}
	m := map[string]any{}
	parent[key] = m
	return m
}

func ensureSlice(parent map[string]any, key string) []any {
	if s, ok := parent[key].([]any); ok {
		return s
	}
	var s []any
	parent[key] = s
	return s
}

func hasDoupassHook(settings map[string]any) bool {
	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		return false
	}
	groups, ok := hooks["PreToolUse"].([]any)
	if !ok {
		return false
	}
	for _, g := range groups {
		group, ok := g.(map[string]any)
		if !ok {
			continue
		}
		handlers, ok := group["hooks"].([]any)
		if !ok {
			continue
		}
		for _, h := range handlers {
			handler, ok := h.(map[string]any)
			if !ok {
				continue
			}
			if cmd, _ := handler["command"].(string); matchesHook(cmd) {
				return true
			}
		}
	}
	return false
}

func matchesHook(command string) bool {
	return strings.Contains(command, hookMarker)
}
