package mcpjson

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/tailscale/hujson"

	"github.com/ogzhncnmr/doupass/internal/fsutil"
)

type Result struct {
	Path    string
	Changed bool
	Backup  string
	Servers int
}

type patchOp struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value,omitempty"`
}

func WrapServers(path string) (Result, error) {
	return patchServers(path, true)
}

func UnwrapServers(path string) (Result, error) {
	return patchServers(path, false)
}

func patchServers(path string, wrap bool) (Result, error) {
	//#nosec G304 -- path is an MCP config file chosen by the local user
	raw, err := os.ReadFile(path)
	if err != nil {
		return Result{Path: path}, err
	}
	doc, err := hujson.Parse(raw)
	if err != nil {
		return Result{Path: path}, fmt.Errorf("mcpjson: %s is not valid JSON: %w", path, err)
	}
	servers, ok := findMember(&doc, "mcpServers")
	if !ok {
		return Result{Path: path}, nil
	}
	members, ok := objectMembers(servers)
	if !ok {
		return Result{Path: path}, nil
	}
	var ops []patchOp
	count := 0
	for i := range members {
		name, ok := stringLiteral(&members[i].Name)
		if !ok {
			continue
		}
		server := &members[i].Value
		commandVal, ok := findMember(server, "command")
		if !ok {
			continue
		}
		command, ok := stringLiteral(commandVal)
		if !ok || command == "" {
			continue
		}
		args := stringArray(findMember(server, "args"))
		if wrap {
			if command == "doupass" {
				continue
			}
			newArgs := append([]string{"proxy", "--server", name, "--", command}, args...)
			ops = append(ops,
				patchOp{Op: "replace", Path: "/mcpServers/" + escapePointer(name) + "/command", Value: "doupass"},
				patchOp{Op: "add", Path: "/mcpServers/" + escapePointer(name) + "/args", Value: newArgs},
			)
			count++
			continue
		}
		if command != "doupass" {
			continue
		}
		idx := -1
		for j, a := range args {
			if a == "--" {
				idx = j
				break
			}
		}
		if idx < 0 || idx+1 >= len(args) {
			continue
		}
		ops = append(ops,
			patchOp{Op: "replace", Path: "/mcpServers/" + escapePointer(name) + "/command", Value: args[idx+1]},
			patchOp{Op: "add", Path: "/mcpServers/" + escapePointer(name) + "/args", Value: args[idx+2:]},
		)
		count++
	}
	if len(ops) == 0 {
		return Result{Path: path}, nil
	}
	patch, err := json.Marshal(ops)
	if err != nil {
		return Result{Path: path}, err
	}
	if err := doc.Patch(patch); err != nil {
		return Result{Path: path}, fmt.Errorf("mcpjson: patch failed: %w", err)
	}
	doc.Format()
	backup := path + ".doupass.bak"
	//#nosec G703 -- backup path derives from the user-selected config path
	if err := os.WriteFile(backup, raw, 0o600); err != nil {
		return Result{Path: path}, err
	}
	//#nosec G703,G306 -- config path is selected by the local user; 0600 protects embedded env secrets
	if err := fsutil.WriteFileAtomic(path, doc.Pack(), 0o600); err != nil {
		return Result{Path: path}, err
	}
	return Result{Path: path, Changed: true, Backup: backup, Servers: count}, nil
}

func escapePointer(s string) string {
	s = strings.ReplaceAll(s, "~", "~0")
	return strings.ReplaceAll(s, "/", "~1")
}

func objectMembers(v *hujson.Value) ([]hujson.ObjectMember, bool) {
	obj, ok := v.Value.(*hujson.Object)
	if !ok {
		return nil, false
	}
	return obj.Members, true
}

func findMember(v *hujson.Value, name string) (*hujson.Value, bool) {
	members, ok := objectMembers(v)
	if !ok {
		return nil, false
	}
	for i := range members {
		if n, ok := stringLiteral(&members[i].Name); ok && n == name {
			return &members[i].Value, true
		}
	}
	return nil, false
}

func stringLiteral(v *hujson.Value) (string, bool) {
	lit, ok := v.Value.(hujson.Literal)
	if !ok || lit.Kind() != '"' {
		return "", false
	}
	return lit.String(), true
}

func stringArray(v *hujson.Value, ok bool) []string {
	if !ok {
		return nil
	}
	arr, isArr := v.Value.(*hujson.Array)
	if !isArr {
		return nil
	}
	out := make([]string, 0, len(arr.Elements))
	for i := range arr.Elements {
		s, isStr := stringLiteral(&arr.Elements[i])
		if !isStr {
			return nil
		}
		out = append(out, s)
	}
	return out
}
