//go:build js && wasm

package main

import (
	"encoding/json"
	"syscall/js"

	"github.com/ogzhncnmr/doupass/internal/policy"
)

type result struct {
	Action string `json:"action,omitempty"`
	Rule   string `json:"rule,omitempty"`
	Reason string `json:"reason,omitempty"`
	Error  string `json:"error,omitempty"`
}

func main() {
	js.Global().Set("doupassDecide", js.FuncOf(decide))
	select {}
}

func decide(_ js.Value, args []js.Value) any {
	if len(args) == 0 {
		return errorResult("missing policy YAML")
	}
	p, err := policy.Parse([]byte(args[0].String()))
	if err != nil {
		return errorResult(err.Error())
	}
	engine, err := policy.New(p, policy.Options{Home: "/home/dev", Workspace: "/workspace"})
	if err != nil {
		return errorResult(err.Error())
	}
	var input struct {
		Surface string         `json:"surface"`
		Server  string         `json:"server"`
		Tool    string         `json:"tool"`
		Args    map[string]any `json:"args"`
	}
	if len(args) > 1 && args[1].String() != "" {
		if err := json.Unmarshal([]byte(args[1].String()), &input); err != nil {
			return errorResult("invalid call JSON: " + err.Error())
		}
	}
	surface := input.Surface
	if surface == "" {
		surface = "hook"
	}
	dec := engine.Decide(policy.Call{Surface: surface, Server: input.Server, Tool: input.Tool, Args: input.Args})
	out, _ := json.Marshal(result{Action: string(dec.Action), Rule: dec.RuleID, Reason: dec.Reason})
	return string(out)
}

func errorResult(msg string) string {
	out, _ := json.Marshal(result{Error: msg})
	return string(out)
}
