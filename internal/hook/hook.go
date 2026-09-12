package hook

import (
	"encoding/json"
	"fmt"

	"github.com/ogzhncnmr/doupass/internal/policy"
)

type Input struct {
	SessionID     string         `json:"session_id" yaml:"session_id"`
	Cwd           string         `json:"cwd" yaml:"cwd"`
	HookEventName string         `json:"hook_event_name" yaml:"hook_event_name"`
	ToolName      string         `json:"tool_name" yaml:"tool_name"`
	ToolInput     map[string]any `json:"tool_input" yaml:"tool_input"`
}

type Output struct {
	HookSpecificOutput *Specific `json:"hookSpecificOutput,omitempty"`
}

type Specific struct {
	HookEventName            string `json:"hookEventName"`
	PermissionDecision       string `json:"permissionDecision"`
	PermissionDecisionReason string `json:"permissionDecisionReason,omitempty"`
}

func ParseInput(data []byte) (Input, error) {
	var in Input
	if err := json.Unmarshal(data, &in); err != nil {
		return Input{}, fmt.Errorf("hook: invalid input JSON: %w", err)
	}
	return in, nil
}

func Decide(engine *policy.Engine, in Input) (Output, policy.Decision) {
	call := policy.Call{Surface: "hook", Tool: in.ToolName, Args: in.ToolInput}
	dec := engine.Decide(call)
	switch dec.Action {
	case policy.ActionDeny, policy.ActionAsk:
		reason := dec.Reason
		if reason == "" {
			reason = "blocked by doupass policy"
		}
		return Output{
			HookSpecificOutput: &Specific{
				HookEventName:            "PreToolUse",
				PermissionDecision:       string(dec.Action),
				PermissionDecisionReason: reason,
			},
		}, dec
	default:
		return Output{}, dec
	}
}
