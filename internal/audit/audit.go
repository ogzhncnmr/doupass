package audit

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ogzhncnmr/doupass/internal/policy"
)

const (
	genesisHash      = "0000000000000000000000000000000000000000000000000000000000000000"
	maxArgValueRunes = 256
	defaultLockWait  = 2 * time.Second
	staleLockAge     = 10 * time.Second
	maxLineBytes     = 10 << 20
)

type Entry struct {
	Seq      int64          `json:"seq"`
	Time     string         `json:"time"`
	Surface  string         `json:"surface,omitempty"`
	Server   string         `json:"server,omitempty"`
	Tool     string         `json:"tool"`
	Args     map[string]any `json:"args,omitempty"`
	Action   string         `json:"action"`
	Rule     string         `json:"rule,omitempty"`
	Reason   string         `json:"reason,omitempty"`
	PrevHash string         `json:"prev_hash"`
	Hash     string         `json:"hash"`
}

type Logger struct {
	Path     string
	LockWait time.Duration
	Now      func() time.Time
	mu       sync.Mutex
}

type Result struct {
	Entries  int
	LastHash string
}

func (l *Logger) Append(call policy.Call, dec policy.Decision) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.Path == "" {
		return errors.New("audit: no path configured")
	}
	if err := os.MkdirAll(filepath.Dir(l.Path), 0o700); err != nil {
		return err
	}
	release, err := acquireLock(l.Path, l.lockWait())
	if err != nil {
		return err
	}
	defer release()

	last, err := readLastEntry(l.Path)
	if err != nil {
		return err
	}
	now := time.Now
	if l.Now != nil {
		now = l.Now
	}
	entry := Entry{
		Seq:      1,
		Time:     now().UTC().Format(time.RFC3339),
		Surface:  call.Surface,
		Server:   call.Server,
		Tool:     call.Tool,
		Args:     maskArgs(call.Args),
		Action:   string(dec.Action),
		Rule:     dec.RuleID,
		Reason:   dec.Reason,
		PrevHash: genesisHash,
	}
	if last != nil {
		entry.Seq = last.Seq + 1
		entry.PrevHash = last.Hash
	}
	entry.Hash, err = entry.computeHash()
	if err != nil {
		return err
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	f, err := os.OpenFile(l.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return err
	}
	return f.Sync()
}

func (l *Logger) lockWait() time.Duration {
	if l.LockWait > 0 {
		return l.LockWait
	}
	return defaultLockWait
}

func (e Entry) computeHash() (string, error) {
	e.Hash = ""
	data, err := json.Marshal(e)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func Verify(path string) (Result, error) {
	entries, err := ReadAll(path)
	if err != nil {
		return Result{}, err
	}
	var res Result
	prev := genesisHash
	for i := range entries {
		e := entries[i]
		want := int64(i + 1)
		if e.Seq != want {
			return res, fmt.Errorf("audit: entry %d: seq %d, want %d", i+1, e.Seq, want)
		}
		if e.PrevHash != prev {
			return res, fmt.Errorf("audit: entry %d: prev_hash mismatch", i+1)
		}
		stored := e.Hash
		computed, err := e.computeHash()
		if err != nil {
			return res, err
		}
		if computed != stored {
			return res, fmt.Errorf("audit: entry %d: hash mismatch (log tampered)", i+1)
		}
		prev = stored
	}
	res.Entries = len(entries)
	res.LastHash = prev
	return res, nil
}

func ReadAll(path string) ([]Entry, error) {
	//#nosec G304 -- path is the audit log configured in the policy
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var entries []Entry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), maxLineBytes)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var e Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			return nil, fmt.Errorf("audit: line %d: %w", lineNo, err)
		}
		entries = append(entries, e)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func readLastEntry(path string) (*Entry, error) {
	//#nosec G304 -- path is the audit log configured in the policy
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var last *Entry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), maxLineBytes)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var e Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			return nil, fmt.Errorf("audit: cannot read last entry: %w", err)
		}
		last = &e
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return last, nil
}

func acquireLock(path string, wait time.Duration) (func(), error) {
	lockPath := path + ".lock"
	deadline := time.Now().Add(wait)
	for {
		//#nosec G304 -- lock file path derives from the configured audit log path
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_ = f.Close()
			return func() { _ = os.Remove(lockPath) }, nil
		}
		if !os.IsExist(err) {
			return nil, err
		}
		if info, statErr := os.Stat(lockPath); statErr == nil && time.Since(info.ModTime()) > staleLockAge {
			_ = os.Remove(lockPath)
			continue
		}
		if time.Now().After(deadline) {
			return nil, errors.New("audit: timed out waiting for log lock")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func maskArgs(args map[string]any) map[string]any {
	if len(args) == 0 {
		return nil
	}
	out := make(map[string]any, len(args))
	for k, v := range args {
		if isSensitiveKey(k) {
			out[k] = "***"
			continue
		}
		switch x := v.(type) {
		case nil:
			out[k] = nil
		case string:
			out[k] = truncate(x)
		default:
			data, err := json.Marshal(x)
			if err != nil {
				out[k] = "[unserializable]"
				continue
			}
			out[k] = truncate(string(data))
		}
	}
	return out
}

func isSensitiveKey(k string) bool {
	lk := strings.ToLower(k)
	if strings.HasSuffix(lk, "key") {
		return true
	}
	for _, s := range []string{"password", "passwd", "secret", "token", "credential"} {
		if strings.Contains(lk, s) {
			return true
		}
	}
	return false
}

func truncate(s string) string {
	if len(s) <= maxArgValueRunes {
		return s
	}
	return s[:maxArgValueRunes]
}
