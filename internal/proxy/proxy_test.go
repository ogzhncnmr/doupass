package proxy

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ogzhncnmr/doupass/internal/policy"
)

const testPolicy = `
version: "0.1"
defaults: {action: allow, ask_fallback: deny, ask_timeout_seconds: 30}
rules:
  - id: block-ssh
    match:
      args:
        "*": "**/.ssh/**"
    action: deny
    reason: "SSH credentials are off-limits"
  - id: ask-write
    match:
      tool: "write_file"
    action: ask
    reason: "writes need approval"
`

const testPolicyAskFallbackAllow = `
version: "0.1"
defaults: {action: allow, ask_fallback: allow}
rules:
  - id: ask-write
    match:
      tool: "write_file"
    action: ask
    reason: "writes need approval"
`

func newEngine(t *testing.T, yaml string) *policy.Engine {
	t.Helper()
	p, err := policy.Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}
	e, err := policy.New(p, policy.Options{Home: "/home/dev", Workspace: "/work"})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	return e
}

type fakeDownstream struct {
	mu    sync.Mutex
	lines []string
}

func (f *fakeDownstream) record(line string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lines = append(f.lines, line)
}

func (f *fakeDownstream) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.lines)
}

func (f *fakeDownstream) toolCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, l := range f.lines {
		if strings.Contains(l, `"tools/call"`) {
			n++
		}
	}
	return n
}

func startFakeServer(t *testing.T) (*io.PipeWriter, *io.PipeReader, *fakeDownstream) {
	t.Helper()
	dInR, dInW := io.Pipe()
	dOutR, dOutW := io.Pipe()
	srv := &fakeDownstream{}
	go func() {
		scanner := bufio.NewScanner(dInR)
		for scanner.Scan() {
			line := scanner.Text()
			srv.record(line)
			var msg rpcMessage
			if err := json.Unmarshal([]byte(line), &msg); err != nil {
				continue
			}
			if msg.Method == "initialize" || msg.Method == "tools/call" {
				resp := fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":{"ok":true}}`, string(msg.ID))
				if _, err := dOutW.Write([]byte(resp + "\n")); err != nil {
					return
				}
			}
		}
		_ = dOutW.Close()
	}()
	t.Cleanup(func() {
		_ = dInR.Close()
		_ = dOutR.Close()
	})
	return dInW, dOutR, srv
}

func readLine(t *testing.T, r *bufio.Reader) string {
	t.Helper()
	type readResult struct {
		s   string
		err error
	}
	ch := make(chan readResult, 1)
	go func() {
		s, err := r.ReadString('\n')
		ch <- readResult{s, err}
	}()
	select {
	case res := <-ch:
		if res.err != nil {
			t.Fatalf("read: %v", res.err)
		}
		return strings.TrimSpace(res.s)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for proxy output")
		return ""
	}
}

type proxyRun struct {
	in     *io.PipeWriter
	out    *bufio.Reader
	done   chan error
	server *fakeDownstream
}

func startProxy(t *testing.T, cfg Config) *proxyRun {
	t.Helper()
	hInR, hInW := io.Pipe()
	hOutR, hOutW := io.Pipe()
	dInW, dOutR, srv := startFakeServer(t)
	done := make(chan error, 1)
	go func() { done <- RunIO(context.Background(), cfg, hInR, hOutW, dInW, dOutR) }()
	t.Cleanup(func() { _ = hOutR.Close() })
	return &proxyRun{in: hInW, out: bufio.NewReader(hOutR), done: done, server: srv}
}

func (p *proxyRun) send(t *testing.T, line string) {
	t.Helper()
	if _, err := p.in.Write([]byte(line + "\n")); err != nil {
		t.Fatalf("send: %v", err)
	}
}

func (p *proxyRun) finish(t *testing.T) error {
	t.Helper()
	_ = p.in.Close()
	select {
	case err := <-p.done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("proxy did not stop")
		return nil
	}
}

func TestAllowsSafeToolCall(t *testing.T) {
	run := startProxy(t, Config{ServerName: "fs", Engine: newEngine(t, testPolicy)})
	run.send(t, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"read_file","arguments":{"path":"/work/README.md"}}}`)
	line := readLine(t, run.out)
	if !strings.Contains(line, `"result"`) {
		t.Fatalf("expected forwarded result, got %s", line)
	}
	if err := run.finish(t); err != nil {
		t.Fatalf("finish: %v", err)
	}
	if got := run.server.toolCalls(); got != 1 {
		t.Fatalf("server tool calls = %d, want 1", got)
	}
}

func TestDeniesCredentialRead(t *testing.T) {
	var got policy.Decision
	run := startProxy(t, Config{
		ServerName: "fs",
		Engine:     newEngine(t, testPolicy),
		OnDecision: func(_ policy.Call, d policy.Decision) { got = d },
	})
	run.send(t, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"read_file","arguments":{"path":"/home/dev/.ssh/id_rsa"}}}`)
	line := readLine(t, run.out)
	if !strings.Contains(line, "-32001") || !strings.Contains(line, "block-ssh") {
		t.Fatalf("expected denial, got %s", line)
	}
	if err := run.finish(t); err != nil {
		t.Fatalf("finish: %v", err)
	}
	if run.server.toolCalls() != 0 {
		t.Fatalf("denied call reached the server")
	}
	if got.Action != policy.ActionDeny || got.RuleID != "block-ssh" {
		t.Fatalf("decision = %+v, want deny/block-ssh", got)
	}
}

func TestAskApprovedForwards(t *testing.T) {
	asked := false
	var got policy.Decision
	run := startProxy(t, Config{
		ServerName: "fs",
		Engine:     newEngine(t, testPolicy),
		Prompt: func(q string) (bool, error) {
			asked = true
			if !strings.Contains(q, "write_file") {
				t.Errorf("question does not mention tool: %s", q)
			}
			return true, nil
		},
		OnDecision: func(_ policy.Call, d policy.Decision) { got = d },
	})
	run.send(t, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"write_file","arguments":{"path":"/work/x"}}}`)
	line := readLine(t, run.out)
	if !strings.Contains(line, `"result"`) {
		t.Fatalf("expected forwarded result, got %s", line)
	}
	if err := run.finish(t); err != nil {
		t.Fatalf("finish: %v", err)
	}
	if !asked {
		t.Fatal("prompt was not called")
	}
	if run.server.toolCalls() != 1 {
		t.Fatalf("approved call was not forwarded")
	}
	if got.Action != policy.ActionAllow || got.RuleID != "ask-write" {
		t.Fatalf("decision = %+v, want allow/ask-write", got)
	}
}

func TestAskDeniedByOperator(t *testing.T) {
	run := startProxy(t, Config{
		ServerName: "fs",
		Engine:     newEngine(t, testPolicy),
		Prompt:     func(string) (bool, error) { return false, nil },
	})
	run.send(t, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"write_file","arguments":{"path":"/work/x"}}}`)
	line := readLine(t, run.out)
	if !strings.Contains(line, "-32001") {
		t.Fatalf("expected denial, got %s", line)
	}
	if err := run.finish(t); err != nil {
		t.Fatalf("finish: %v", err)
	}
	if run.server.toolCalls() != 0 {
		t.Fatal("denied call reached the server")
	}
}

func TestAskWithoutPromptUsesFallbackDeny(t *testing.T) {
	run := startProxy(t, Config{ServerName: "fs", Engine: newEngine(t, testPolicy)})
	run.send(t, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"write_file","arguments":{"path":"/work/x"}}}`)
	line := readLine(t, run.out)
	if !strings.Contains(line, "-32001") {
		t.Fatalf("expected fail-closed denial, got %s", line)
	}
	if err := run.finish(t); err != nil {
		t.Fatalf("finish: %v", err)
	}
}

func TestAskFallbackAllow(t *testing.T) {
	run := startProxy(t, Config{ServerName: "fs", Engine: newEngine(t, testPolicyAskFallbackAllow)})
	run.send(t, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"write_file","arguments":{"path":"/work/x"}}}`)
	line := readLine(t, run.out)
	if !strings.Contains(line, `"result"`) {
		t.Fatalf("expected forwarded result with allow fallback, got %s", line)
	}
	if err := run.finish(t); err != nil {
		t.Fatalf("finish: %v", err)
	}
	if run.server.toolCalls() != 1 {
		t.Fatal("call was not forwarded")
	}
}

func TestNonToolCallPassesThrough(t *testing.T) {
	run := startProxy(t, Config{ServerName: "fs", Engine: newEngine(t, testPolicy)})
	run.send(t, `{"jsonrpc":"2.0","id":10,"method":"initialize","params":{}}`)
	line := readLine(t, run.out)
	if !strings.Contains(line, `"result"`) {
		t.Fatalf("expected forwarded response, got %s", line)
	}
	if err := run.finish(t); err != nil {
		t.Fatalf("finish: %v", err)
	}
	if run.server.count() != 1 {
		t.Fatalf("server received %d lines, want 1", run.server.count())
	}
}

func TestInvalidJSONRejected(t *testing.T) {
	var got policy.Decision
	run := startProxy(t, Config{
		ServerName: "fs",
		Engine:     newEngine(t, testPolicy),
		OnDecision: func(_ policy.Call, d policy.Decision) { got = d },
	})
	run.send(t, "this is not json")
	line := readLine(t, run.out)
	if !strings.Contains(line, "-32700") {
		t.Fatalf("expected parse error, got %s", line)
	}
	if err := run.finish(t); err != nil {
		t.Fatalf("finish: %v", err)
	}
	if run.server.count() != 0 {
		t.Fatalf("unparseable line was forwarded downstream")
	}
	if got.Action != policy.ActionDeny {
		t.Fatalf("decision = %+v, want deny", got)
	}
}

func TestBatchRequestRejected(t *testing.T) {
	run := startProxy(t, Config{ServerName: "fs", Engine: newEngine(t, testPolicy)})
	run.send(t, `[{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"read_file","arguments":{"path":"/home/dev/.ssh/id_rsa"}}}]`)
	line := readLine(t, run.out)
	if !strings.Contains(line, "-32600") {
		t.Fatalf("expected invalid request error for batch, got %s", line)
	}
	if err := run.finish(t); err != nil {
		t.Fatalf("finish: %v", err)
	}
	if run.server.count() != 0 {
		t.Fatal("batch message was forwarded downstream")
	}
}

func TestMissingToolNameRejected(t *testing.T) {
	run := startProxy(t, Config{ServerName: "fs", Engine: newEngine(t, testPolicy)})
	run.send(t, `{"jsonrpc":"2.0","id":12,"method":"tools/call","params":{}}`)
	line := readLine(t, run.out)
	if !strings.Contains(line, "-32602") {
		t.Fatalf("expected invalid params error, got %s", line)
	}
	if err := run.finish(t); err != nil {
		t.Fatalf("finish: %v", err)
	}
	if run.server.count() != 0 {
		t.Fatal("nameless tools/call was forwarded downstream")
	}
}

func TestNonObjectArgumentsRejected(t *testing.T) {
	run := startProxy(t, Config{ServerName: "fs", Engine: newEngine(t, testPolicy)})
	run.send(t, `{"jsonrpc":"2.0","id":13,"method":"tools/call","params":{"name":"read_file","arguments":"nope"}}`)
	line := readLine(t, run.out)
	if !strings.Contains(line, "-32602") {
		t.Fatalf("expected invalid params error, got %s", line)
	}
	if err := run.finish(t); err != nil {
		t.Fatalf("finish: %v", err)
	}
	if run.server.count() != 0 {
		t.Fatal("call with non-object arguments was forwarded downstream")
	}
}

func TestOversizedMessageRejected(t *testing.T) {
	run := startProxy(t, Config{ServerName: "fs", Engine: newEngine(t, testPolicy), MaxMessageBytes: 256})
	big := `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"read_file","arguments":{"path":"/work/` + strings.Repeat("a", 512) + `"}}}`
	go func() { _, _ = run.in.Write([]byte(big + "\n")) }()
	line := readLine(t, run.out)
	if !strings.Contains(line, "-32002") {
		t.Fatalf("expected protocol error, got %s", line)
	}
	select {
	case err := <-run.done:
		if err == nil {
			t.Fatal("expected RunIO to fail on oversized message")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("proxy did not stop")
	}
}

func TestRunWithRealProcess(t *testing.T) {
	engine := newEngine(t, testPolicy)
	hInR, hInW := io.Pipe()
	hOutR, hOutW := io.Pipe()
	cfg := Config{
		ServerName: "fs",
		Engine:     engine,
		Command:    []string{os.Args[0], "-test.run=TestHelperProcess"},
		Env:        append(os.Environ(), "DOUPASS_HELPER=1"),
		Stdin:      hInR,
		Stdout:     hOutW,
	}
	done := make(chan error, 1)
	go func() { done <- Run(context.Background(), cfg) }()
	if _, err := hInW.Write([]byte(`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"read_file","arguments":{"path":"/work/README.md"}}}` + "\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	line := readLine(t, bufio.NewReader(hOutR))
	if !strings.Contains(line, `"result"`) {
		t.Fatalf("expected result from real process, got %s", line)
	}
	_ = hInW.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not stop")
	}
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("DOUPASS_HELPER") != "1" {
		t.Skip("helper process")
	}
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var msg rpcMessage
		if json.Unmarshal(scanner.Bytes(), &msg) != nil {
			continue
		}
		if msg.Method == "tools/call" {
			fmt.Printf(`{"jsonrpc":"2.0","id":%s,"result":{"ok":true}}`+"\n", string(msg.ID))
		}
	}
	os.Exit(0)
}
