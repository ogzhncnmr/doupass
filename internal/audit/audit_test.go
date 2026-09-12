package audit

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ogzhncnmr/doupass/internal/policy"
)

func appendEntry(t *testing.T, l *Logger, i int) {
	t.Helper()
	call := policy.Call{Surface: "hook", Tool: "Bash", Args: map[string]any{"command": "ls"}}
	dec := policy.Decision{Action: policy.ActionAllow}
	if i%2 == 0 {
		dec = policy.Decision{Action: policy.ActionDeny, RuleID: "rule-x", Reason: "nope"}
	}
	if err := l.Append(call, dec); err != nil {
		t.Fatalf("append: %v", err)
	}
}

func TestAppendAndVerify(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	l := &Logger{Path: path}
	for i := 0; i < 3; i++ {
		appendEntry(t, l, i)
	}
	res, err := Verify(path)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if res.Entries != 3 {
		t.Fatalf("entries = %d, want 3", res.Entries)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("lines = %d, want 3", len(lines))
	}
}

func TestMaskingAndTruncation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	l := &Logger{Path: path}
	call := policy.Call{
		Tool: "Bash",
		Args: map[string]any{
			"password": "hunter2",
			"apiKey":   "sk-123",
			"command":  strings.Repeat("x", 400),
			"list":     []any{"a", "b"},
		},
	}
	if err := l.Append(call, policy.Decision{Action: policy.ActionAllow}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	text := string(data)
	if strings.Contains(text, "hunter2") || strings.Contains(text, "sk-123") {
		t.Fatal("sensitive values leaked into the audit log")
	}
	if !strings.Contains(text, `"***"`) {
		t.Fatal("masking marker missing")
	}
	entries, err := ReadAll(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %d", len(entries))
	}
	if got := entries[0].Args["command"].(string); len(got) != 256 {
		t.Fatalf("command truncated to %d, want 256", len(got))
	}
	if entries[0].Args["list"] != `["a","b"]` {
		t.Fatalf("list arg = %v", entries[0].Args["list"])
	}
}

func TestTamperDetected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	l := &Logger{Path: path}
	appendEntry(t, l, 0)
	appendEntry(t, l, 1)
	data, _ := os.ReadFile(path)
	tampered := strings.Replace(string(data), `"action":"allow"`, `"action":"deny"`, 1)
	if tampered == string(data) {
		t.Fatal("test setup failed to modify the log")
	}
	if err := os.WriteFile(path, []byte(tampered), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(path); err == nil {
		t.Fatal("expected tamper detection, got nil")
	}
}

func TestCorruptLineDetected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	if err := os.WriteFile(path, []byte("not json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(path); err == nil {
		t.Fatal("expected error for corrupt log")
	}
}

func TestMissingFileIsValid(t *testing.T) {
	res, err := Verify(filepath.Join(t.TempDir(), "absent.jsonl"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Entries != 0 {
		t.Fatalf("entries = %d, want 0", res.Entries)
	}
}

func TestConcurrentAppends(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for w := 0; w < 2; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l := &Logger{Path: path}
			for i := 0; i < 20; i++ {
				call := policy.Call{Tool: "Bash"}
				if err := l.Append(call, policy.Decision{Action: policy.ActionAllow}); err != nil {
					errs <- err
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent append: %v", err)
	}
	entries, err := ReadAll(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 40 {
		t.Fatalf("entries = %d, want 40", len(entries))
	}
	for i, e := range entries {
		if e.Seq != int64(i+1) {
			t.Fatalf("entry %d has seq %d", i, e.Seq)
		}
	}
	if _, err := Verify(path); err != nil {
		t.Fatalf("verify after concurrent appends: %v", err)
	}
}

func TestLockTimeout(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")
	lockPath := path + ".lock"
	if err := os.WriteFile(lockPath, []byte("held"), 0o600); err != nil {
		t.Fatal(err)
	}
	l := &Logger{Path: path, LockWait: 100 * time.Millisecond}
	err := l.Append(policy.Call{Tool: "Bash"}, policy.Decision{Action: policy.ActionAllow})
	if err == nil || !strings.Contains(err.Error(), "lock") {
		t.Fatalf("expected lock timeout, got %v", err)
	}
}

func TestStaleLockRecovered(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")
	lockPath := path + ".lock"
	if err := os.WriteFile(lockPath, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Minute)
	if err := os.Chtimes(lockPath, old, old); err != nil {
		t.Fatal(err)
	}
	l := &Logger{Path: path, LockWait: time.Second}
	if err := l.Append(policy.Call{Tool: "Bash"}, policy.Decision{Action: policy.ActionAllow}); err != nil {
		t.Fatalf("stale lock not recovered: %v", err)
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatal("lock file should be removed after append")
	}
}

func TestNoPathConfigured(t *testing.T) {
	l := &Logger{}
	if err := l.Append(policy.Call{}, policy.Decision{}); err == nil {
		t.Fatal("expected error when path is empty")
	}
}
