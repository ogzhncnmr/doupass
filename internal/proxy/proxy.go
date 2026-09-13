package proxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/ogzhncnmr/doupass/internal/policy"
)

const (
	defaultMaxMessageBytes   = 10 << 20
	deniedErrorCode          = -32001
	protocolErrorCode        = -32002
	invalidParamsErrorCode   = -32602
	invalidRequestErrorCode  = -32600
	parseErrorCode           = -32700
	defaultAskTimeoutSeconds = 60
)

type Config struct {
	ServerName      string
	Engine          *policy.Engine
	Command         []string
	Env             []string
	Stdin           io.Reader
	Stdout          io.Writer
	Stderr          io.Writer
	Prompt          func(question string) (bool, error)
	OnDecision      func(call policy.Call, dec policy.Decision)
	MaxMessageBytes int
}

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type callParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type errorResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Error   rpcError        `json:"error"`
}

type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

func Run(ctx context.Context, cfg Config) error {
	if len(cfg.Command) == 0 {
		return errors.New("proxy: no downstream command configured")
	}
	//#nosec G204 -- the downstream MCP server command is configured by the local user
	cmd := exec.CommandContext(ctx, cfg.Command[0], cfg.Command[1:]...)
	if cfg.Env != nil {
		cmd.Env = cfg.Env
	}
	if cfg.Stderr != nil {
		cmd.Stderr = cfg.Stderr
	} else {
		cmd.Stderr = os.Stderr
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	pg := newProcessGroup()
	defer pg.close()
	pg.configure(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := pg.attach(cmd); err != nil {
		pg.detach()
	}
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			pg.kill(cmd)
		case <-done:
		}
	}()

	in := cfg.Stdin
	if in == nil {
		in = os.Stdin
	}
	out := cfg.Stdout
	if out == nil {
		out = os.Stdout
	}
	runErr := RunIO(ctx, cfg, in, out, stdin, stdout)
	_ = stdin.Close()
	waitErr := cmd.Wait()
	close(done)
	if runErr != nil {
		return runErr
	}
	return waitErr
}

func RunIO(ctx context.Context, cfg Config, harnessIn io.Reader, harnessOut io.Writer, downIn io.Writer, downOut io.Reader) error {
	out := &lockedWriter{w: harnessOut}
	copied := make(chan error, 1)
	go func() {
		_, err := io.Copy(out, downOut)
		copied <- err
	}()

	scanner := bufio.NewScanner(harnessIn)
	max := cfg.MaxMessageBytes
	if max <= 0 {
		max = defaultMaxMessageBytes
	}
	initial := 64 * 1024
	if max < initial {
		initial = max
	}
	scanner.Buffer(make([]byte, initial), max)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		forward, ok := handleMessage(ctx, &cfg, out, line)
		if !ok {
			continue
		}
		buf := make([]byte, 0, len(forward)+1)
		buf = append(buf, forward...)
		buf = append(buf, '\n')
		if _, err := downIn.Write(buf); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		writeRPCError(out, nil, protocolErrorCode, "doupass: "+err.Error())
		return err
	}

	if closer, ok := downIn.(io.Closer); ok {
		_ = closer.Close()
	}
	select {
	case err := <-copied:
		if err != nil && !errors.Is(err, io.ErrClosedPipe) {
			return err
		}
	case <-time.After(5 * time.Second):
	case <-ctx.Done():
	}
	return nil
}

func handleMessage(ctx context.Context, cfg *Config, out io.Writer, line []byte) ([]byte, bool) {
	if trimmed := bytes.TrimSpace(line); len(trimmed) > 0 && trimmed[0] == '[' {
		return cfg.reject(out, nil, invalidRequestErrorCode, "batch requests are not supported over stdio; message not forwarded")
	}
	var msg rpcMessage
	if err := json.Unmarshal(line, &msg); err != nil {
		return cfg.reject(out, nil, parseErrorCode, "unparseable JSON-RPC message not forwarded: "+err.Error())
	}
	if msg.Method != "tools/call" {
		return line, true
	}
	var params callParams
	if len(msg.Params) > 0 {
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			return cfg.reject(out, msg.ID, invalidParamsErrorCode, "unparseable tools/call params not forwarded: "+err.Error())
		}
	}
	if params.Name == "" {
		return cfg.reject(out, msg.ID, invalidParamsErrorCode, "tools/call without a tool name not forwarded")
	}

	call := policy.Call{
		Surface: "mcp",
		Server:  cfg.ServerName,
		Tool:    params.Name,
		Args:    params.Arguments,
	}
	dec := cfg.Engine.Decide(call)

	if dec.Action == policy.ActionAsk {
		resolved, note := resolveAsk(ctx, cfg, params.Name, dec)
		dec.Action = resolved
		if note != "" {
			if dec.Reason != "" {
				dec.Reason = dec.Reason + "; " + note
			} else {
				dec.Reason = note
			}
		}
	}
	if cfg.OnDecision != nil {
		cfg.OnDecision(call, dec)
	}

	switch dec.Action {
	case policy.ActionAllow, policy.ActionLog:
		return line, true
	default:
		writeRPCError(out, msg.ID, deniedErrorCode, denyMessage(cfg.ServerName, params.Name, dec))
		return nil, false
	}
}

// reject fails closed on messages the engine cannot evaluate: the caller gets
// a JSON-RPC error and the message is never forwarded downstream.
func (cfg *Config) reject(out io.Writer, id json.RawMessage, code int, reason string) ([]byte, bool) {
	if cfg.OnDecision != nil {
		call := policy.Call{Surface: "mcp", Server: cfg.ServerName, Tool: "<unforwarded>"}
		cfg.OnDecision(call, policy.Decision{Action: policy.ActionDeny, Reason: reason})
	}
	writeRPCError(out, id, code, "doupass: "+reason)
	return nil, false
}

func denyMessage(server, tool string, dec policy.Decision) string {
	target := tool
	if server != "" {
		target = server + "." + tool
	}
	var b strings.Builder
	b.WriteString("doupass denied ")
	b.WriteString(target)
	if dec.RuleID != "" {
		b.WriteString(" by rule ")
		b.WriteString(dec.RuleID)
	} else {
		b.WriteString(" by policy")
	}
	if dec.Reason != "" {
		b.WriteString(": ")
		b.WriteString(dec.Reason)
	}
	return b.String()
}

func resolveAsk(ctx context.Context, cfg *Config, tool string, dec policy.Decision) (policy.Action, string) {
	fallback := cfg.Engine.Policy.Defaults.AskFallback
	if fallback == "" {
		fallback = policy.ActionDeny
	}
	if cfg.Prompt == nil {
		return fallback, "no interactive prompt available; fallback applied"
	}
	timeout := time.Duration(cfg.Engine.Policy.Defaults.AskTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = defaultAskTimeoutSeconds * time.Second
	}
	question := fmt.Sprintf("doupass: allow %s tool %q? (%s) [y/N] ", cfg.ServerName, tool, dec.Reason)
	type result struct {
		ok  bool
		err error
	}
	ch := make(chan result, 1)
	go func() {
		ok, err := cfg.Prompt(question)
		ch <- result{ok: ok, err: err}
	}()
	select {
	case r := <-ch:
		if r.err != nil || !r.ok {
			return policy.ActionDeny, "denied interactively"
		}
		return policy.ActionAllow, "approved interactively"
	case <-time.After(timeout):
		return fallback, "ask timed out; fallback applied"
	case <-ctx.Done():
		return policy.ActionDeny, "cancelled"
	}
}

func writeRPCError(w io.Writer, id json.RawMessage, code int, message string) {
	resp := errorResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   rpcError{Code: code, Message: message},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}
	data = append(data, '\n')
	_, _ = w.Write(data)
}
