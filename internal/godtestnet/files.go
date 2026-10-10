//go:build go1.25

// Package godtestnet provisions private synthetic node workspaces. It never
// contacts RH, creates backing, deploys a public service or resets a signer.
package godtestnet

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"

	cmtjson "github.com/cometbft/cometbft/libs/json"
)

var (
	ErrPrivate = errors.New("private testnet workspace rejected")
	ErrConfig  = errors.New("synthetic testnet configuration rejected")
	ErrStart   = errors.New("synthetic testnet node could not start")
)

// Disk operations stay disabled on Windows until owner-only ACLs are tested.
// OpenRoot confines accesses even if an intermediate path changes concurrently.
func openPrivate(home string) (*os.Root, error) {
	if runtime.GOOS == "windows" || !filepath.IsAbs(home) || filepath.Clean(home) == string(filepath.Separator) {
		return nil, ErrPrivate
	}
	s, err := os.Lstat(home)
	if err != nil || !s.IsDir() || s.Mode()&os.ModeSymlink != 0 || s.Mode().Perm()&0077 != 0 {
		return nil, ErrPrivate
	}
	r, err := os.OpenRoot(home)
	if err != nil {
		return nil, ErrPrivate
	}
	opened, err := r.Stat(".")
	if err != nil || !os.SameFile(s, opened) || opened.Mode().Perm()&0077 != 0 {
		_ = r.Close()
		return nil, ErrPrivate
	}
	return r, nil
}

func readPrivate(root *os.Root, name string, limit int64) ([]byte, error) {
	s, err := root.Lstat(name)
	if err != nil || !s.Mode().IsRegular() || s.Mode().Perm()&0077 != 0 || s.Size() <= 0 || s.Size() > limit {
		return nil, ErrPrivate
	}
	f, err := root.Open(name)
	if err != nil {
		return nil, ErrPrivate
	}
	defer f.Close()
	actual, err := f.Stat()
	if err != nil || !os.SameFile(s, actual) {
		return nil, ErrPrivate
	}
	raw, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || int64(len(raw)) > limit {
		return nil, ErrPrivate
	}
	return raw, nil
}

// Exclusive creation never overwrites existing operational or signing state.
// Sync the new file and directory before reporting successful initialization.
func writePrivate(root *os.Root, name string, raw []byte) error {
	f, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return ErrPrivate
	}
	_, err = f.Write(raw)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		return ErrPrivate
	}
	d, err := root.Open(".")
	if err != nil {
		return ErrPrivate
	}
	defer d.Close()
	if d.Sync() != nil {
		return ErrPrivate
	}
	return nil
}

// Strict JSON rejects duplicate keys recursively, case aliases, unknown fields,
// trailing documents and excessive nesting. Errors never echo private input.
func decode(raw []byte, out any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var visit func(int) error
	visit = func(depth int) error {
		if depth > 16 {
			return ErrConfig
		}
		t, err := d.Token()
		if err != nil {
			return ErrConfig
		}
		if delim, ok := t.(json.Delim); ok {
			switch delim {
			case '{':
				seen := map[string]bool{}
				for d.More() {
					k, err := d.Token()
					key, ok := k.(string)
					if err != nil || !ok || seen[key] {
						return ErrConfig
					}
					seen[key] = true
					if visit(depth+1) != nil {
						return ErrConfig
					}
				}
			case '[':
				for d.More() {
					if visit(depth+1) != nil {
						return ErrConfig
					}
				}
			default:
				return ErrConfig
			}
			end, err := d.Token()
			if err != nil || delim == '{' && end != json.Delim('}') || delim == '[' && end != json.Delim(']') {
				return ErrConfig
			}
		}
		return nil
	}
	if visit(0) != nil {
		return ErrConfig
	}
	if _, err := d.Token(); err != io.EOF {
		return ErrConfig
	}
	d = json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(out) != nil || d.Decode(new(any)) != io.EOF {
		return ErrConfig
	}
	// Encoding/json accepts case-insensitive names. Canonicalize field names
	// by comparing keys with a freshly marshalled instance at every depth.
	canonical, err := json.Marshal(out)
	if err != nil {
		return ErrConfig
	}
	return canonicalKeys(raw, canonical)
}

// The consensus library's interface decoder alone permits duplicate keys and
// unknown fields. Apply the same bounded syntax and exact-key checks to its
// key, signing-progress and genesis formats without changing those formats.
func decodeConsensus(raw []byte, out any) error {
	var syntax json.RawMessage
	if decode(raw, &syntax) != nil || cmtjson.Unmarshal(raw, out) != nil {
		return ErrPrivate
	}
	canonical, err := cmtjson.Marshal(out)
	if err != nil || canonicalKeys(raw, canonical) != nil {
		return ErrPrivate
	}
	return nil
}

func canonicalKeys(raw, canonical []byte) error {
	var have, want any
	if json.Unmarshal(raw, &have) != nil || json.Unmarshal(canonical, &want) != nil {
		return ErrConfig
	}
	var keys func(any, any) bool
	keys = func(a, b any) bool {
		switch x := a.(type) {
		case map[string]any:
			y, ok := b.(map[string]any)
			if !ok {
				return false
			}
			for k, v := range x {
				w, ok := y[k]
				if !ok || !keys(v, w) {
					return false
				}
			}
		case []any:
			y, ok := b.([]any)
			if !ok || len(x) != len(y) {
				return false
			}
			for i := range x {
				if !keys(x[i], y[i]) {
					return false
				}
			}
		}
		return true
	}
	if !keys(have, want) {
		return ErrConfig
	}
	return nil
}
