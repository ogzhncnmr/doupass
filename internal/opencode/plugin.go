package opencode

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const pluginMarker = "managed by doupass"

type PluginResult struct {
	Path    string
	Changed bool
}

func InstallPlugin(pluginPath, binary string) (PluginResult, error) {
	//#nosec G304 -- plugin path is chosen by the local user via --plugin-dir
	if data, err := os.ReadFile(pluginPath); err == nil {
		if strings.Contains(string(data), pluginMarker) {
			return PluginResult{Path: pluginPath}, nil
		}
		return PluginResult{Path: pluginPath}, fmt.Errorf("opencode: %s already exists and is not managed by doupass", pluginPath)
	} else if !os.IsNotExist(err) {
		return PluginResult{Path: pluginPath}, err
	}
	if binary == "" {
		binary = "doupass"
	}
	if err := os.MkdirAll(filepath.Dir(pluginPath), 0o750); err != nil {
		return PluginResult{Path: pluginPath}, err
	}
	//#nosec G306 -- plugin file contains no secrets; 0600 keeps it user-owned
	if err := os.WriteFile(pluginPath, []byte(renderPlugin(binary)), 0o600); err != nil {
		return PluginResult{Path: pluginPath}, err
	}
	return PluginResult{Path: pluginPath, Changed: true}, nil
}

func UninstallPlugin(pluginPath string) (PluginResult, error) {
	//#nosec G304 -- plugin path is chosen by the local user via --plugin-dir
	data, err := os.ReadFile(pluginPath)
	if os.IsNotExist(err) {
		return PluginResult{Path: pluginPath}, nil
	}
	if err != nil {
		return PluginResult{Path: pluginPath}, err
	}
	if !strings.Contains(string(data), pluginMarker) {
		return PluginResult{Path: pluginPath}, nil
	}
	if err := os.Remove(pluginPath); err != nil {
		return PluginResult{Path: pluginPath}, err
	}
	return PluginResult{Path: pluginPath, Changed: true}, nil
}

func renderPlugin(binary string) string {
	binaryLiteral := strings.ReplaceAll(binary, `\`, `\\`)
	binaryLiteral = strings.ReplaceAll(binaryLiteral, `"`, `\"`)
	return `// ` + pluginMarker + ` (doupass install opencode --plugin)
import { writeFileSync, unlinkSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"

const BINARY = "` + binaryLiteral + `"

function canonicalTool(name) {
  const map = {
    read: "Read", write: "Write", edit: "Edit", bash: "Bash", glob: "Glob",
    grep: "Grep", list: "List", webfetch: "WebFetch", websearch: "WebSearch",
    task: "Task", todowrite: "TodoWrite",
  }
  return map[name] ?? name.charAt(0).toUpperCase() + name.slice(1)
}

function normalizeArgs(args) {
  if (!args || typeof args !== "object") return {}
  const out = { ...args }
  if (out.filePath !== undefined && out.file_path === undefined) out.file_path = out.filePath
  return out
}

export const DoupassPlugin = async ({ $ }) => {
  return {
    "tool.execute.before": async (input, output) => {
      const payload = JSON.stringify({
        surface: "hook",
        tool: canonicalTool(input.tool),
        args: normalizeArgs(output.args),
      })
      const file = join(tmpdir(), "doupass-" + Date.now() + "-" + Math.random().toString(36).slice(2) + ".json")
      let decision
      try {
        writeFileSync(file, payload)
        const text = await $` + "`" + `${BINARY} decide --input ${file}` + "`" + `.quiet().text()
        decision = JSON.parse(text)
      } catch (err) {
        throw new Error("doupass: policy decision failed (fail-closed): " + err)
      } finally {
        try { unlinkSync(file) } catch {}
      }
      if (decision.action === "deny") {
        throw new Error("doupass denied " + input.tool + " by rule " + (decision.rule || "policy") + ": " + (decision.reason || ""))
      }
      if (decision.action === "ask") {
        throw new Error("doupass: " + input.tool + " requires approval (ask is fail-closed on opencode) - rule " + (decision.rule || "policy"))
      }
    },
  }
}
`
}
