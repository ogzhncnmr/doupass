package opencode

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/tailscale/hujson"
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
	raw, err := os.ReadFile(path)
	if err != nil {
		return Result{Path: path}, err
	}
	doc, err := hujson.Parse(raw)
	if err != nil {
		return Result{Path: path}, fmt.Errorf("opencode: %s is not valid JSON: %w", path, err)
	}
	mcp, ok := findMember(&doc, "mcp")
	if !ok {
		return Result{Path: path}, nil
	}
	members, ok := objectMembers(mcp)
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
		if typ, ok := findMember(server, "type"); !ok {
			continue
		} else if typeName, ok := stringLiteral(typ); !ok || typeName != "local" {
			continue
		}
		commandVal, ok := findMember(server, "command")
		if !ok {
			continue
		}
		command, ok := stringArray(commandVal)
		if !ok || len(command) == 0 {
			continue
		}
		isWrapped := command[0] == "doupass"
		if wrap && isWrapped {
			continue
		}
		if !wrap && !isWrapped {
			continue
		}
		newCommand := wrapCommand(name, command)
		if !wrap {
			newCommand = unwrapCommand(command)
			if newCommand == nil {
				continue
			}
		}
		ops = append(ops, patchOp{
			Op:    "replace",
			Path:  "/mcp/" + escapePointer(name) + "/command",
			Value: newCommand,
		})
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
		return Result{Path: path}, fmt.Errorf("opencode: patch failed: %w", err)
	}
	doc.Format()
	backup := path + ".doupass.bak"
	if err := os.WriteFile(backup, raw, 0o600); err != nil {
		return Result{Path: path}, err
	}
	if err := os.WriteFile(path, doc.Pack(), 0o644); err != nil {
		return Result{Path: path}, err
	}
	return Result{Path: path, Changed: true, Backup: backup, Servers: count}, nil
}

func wrapCommand(server string, command []string) []string {
	wrapped := []string{"doupass", "proxy", "--server", server, "--"}
	return append(wrapped, command...)
}

func unwrapCommand(command []string) []string {
	for i, c := range command {
		if c == "--" {
			if i == len(command)-1 {
				return nil
			}
			return command[i+1:]
		}
	}
	return nil
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

func stringArray(v *hujson.Value) ([]string, bool) {
	arr, ok := v.Value.(*hujson.Array)
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(arr.Elements))
	for i := range arr.Elements {
		s, ok := stringLiteral(&arr.Elements[i])
		if !ok {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}
