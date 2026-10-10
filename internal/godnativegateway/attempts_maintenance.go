//go:build go1.25

package godnativegateway

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"strings"

	bolt "go.etcd.io/bbolt"
)

// AttemptAudit contains only aggregate inventory and checksums. It is private
// operational evidence, not a public status route or an admission certificate.
type AttemptAudit struct {
	SchemaVersion int    `json:"schemaVersion"`
	PagesChecked  bool   `json:"pagesChecked"`
	Records       uint64 `json:"records"`
	Capacity      uint64 `json:"capacity"`
	Remaining     uint64 `json:"remaining"`
	FileBytes     int64  `json:"fileBytes"`
	FileBudget    int64  `json:"fileBudget"`
	LogicalSHA256 string `json:"logicalSHA256"`
	FileSHA256    string `json:"fileSHA256"`
}

type AttemptCopyAudit struct {
	Source      AttemptAudit `json:"source"`
	Destination AttemptAudit `json:"destination"`
}

func (s *attemptStore) audit() (out AttemptAudit, err error) {
	defer func() {
		if recover() != nil {
			out, err = AttemptAudit{}, ErrGateway
		}
	}()
	if !s.fileOK() || s.count > s.capacity {
		return out, ErrGateway
	}
	f, err := s.root.OpenFile(attemptFile, os.O_RDONLY|attemptNoFollow(), 0)
	if err != nil {
		return out, ErrGateway
	}
	defer f.Close()
	before, err := f.Stat()
	if err != nil || !attemptOwner(before, false) || !os.SameFile(before, s.file) {
		return out, ErrGateway
	}
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, s.maxFile+1))
	after, e := f.Stat()
	if err != nil || e != nil || n != before.Size() || n > s.maxFile || after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) || !s.fileOK() {
		return out, ErrGateway
	}
	var policy attemptIdentity
	if json.Unmarshal(s.policy, &policy) != nil {
		return out, ErrGateway
	}
	out = AttemptAudit{SchemaVersion: policy.Version, Records: s.count, Capacity: s.capacity, Remaining: s.capacity - s.count, FileBytes: n, FileBudget: s.maxFile, LogicalSHA256: s.logical, FileSHA256: hex.EncodeToString(h.Sum(nil))}
	return out, nil
}

// AuditAttempts exclusively locks an existing store, scans every record, and
// hashes the retained file. It cannot run against a live writer, initialize a
// missing store, repair corruption, read keys, call RPC or start a listener.
func AuditAttempts(c Config) (out AttemptAudit, err error) {
	return auditAttempts(c, false)
}

// CheckAttempts adds the pinned database's physical consistency check. It must
// run in an isolated, supervised offline process: the upstream checker walks
// mmap pages in its own goroutine and malformed storage can terminate that
// process. Never call it from a gateway, node, or an in-process recovery hook.
// Its diagnostics are discarded; only the aggregate or ErrGateway is returned.
func CheckAttempts(c Config) (out AttemptAudit, err error) {
	return auditAttempts(c, true)
}

type attemptRedactor struct{}

func (attemptRedactor) KeyToString([]byte) string   { return "redacted" }
func (attemptRedactor) ValueToString([]byte) string { return "redacted" }

func auditAttempts(c Config, physical bool) (out AttemptAudit, err error) {
	if !validAttemptPolicy(c) {
		return out, ErrGateway
	}
	s, err := openAttempts(c, false, nil, nil)
	if err != nil {
		return out, ErrGateway
	}
	defer func() {
		if s.close() != nil || err != nil {
			out, err = AttemptAudit{}, ErrGateway
		}
	}()
	if physical {
		err = s.db.View(func(tx *bolt.Tx) error {
			failed := false
			// Drain the channel before the read transaction closes, including
			// after an error. Otherwise the checker can block on a send or walk
			// pages after the transaction has released them.
			for range tx.Check(bolt.WithKVStringer(attemptRedactor{})) {
				failed = true
			}
			if failed {
				return ErrGateway
			}
			return nil
		})
		if err != nil {
			return AttemptAudit{}, ErrGateway
		}
	}
	out, err = s.audit()
	if err == nil {
		out.PagesChecked = physical
	}
	return out, err
}

// CopyAttempts produces a new offline copy at equal or larger capacity. The
// caller must review a complete source audit and pin its exact file checksum.
// This preserves ALL existing hashes, including unknown, submitted, confirmed
// and failed attempts. It does not reconcile a legacy memory-only gateway.
// No overwrite, in-place migration, automatic activation or deletion occurs.
func CopyAttempts(c Config, destination string, capacity uint64, expectedSourceSHA256 string) (out AttemptCopyAudit, err error) {
	if !validAttemptPolicy(c) || capacity < attemptCapacity(c) || !validAttemptCapacity(capacity) || len(expectedSourceSHA256) != 64 || strings.Trim(expectedSourceSHA256, "0123456789abcdef") != "" {
		return out, ErrGateway
	}
	source, err := openAttempts(c, false, nil, nil)
	if err != nil {
		return out, ErrGateway
	}
	defer func() {
		if source.close() != nil || err != nil {
			out, err = AttemptCopyAudit{}, ErrGateway
		}
	}()
	before, err := source.audit()
	if err != nil || before.FileSHA256 != expectedSourceSHA256 {
		return out, ErrGateway
	}
	targetConfig := c
	targetConfig.AttemptDirectory, targetConfig.AttemptCapacity = destination, capacity
	target, err := openAttempts(targetConfig, true, nil, source)
	if err != nil {
		return out, ErrGateway
	}
	defer func() {
		if target.close() != nil || err != nil {
			out, err = AttemptCopyAudit{}, ErrGateway
		}
	}()
	after, err := source.audit()
	if err != nil || after != before {
		return out, ErrGateway
	}
	copyAudit, err := target.audit()
	if err != nil || copyAudit.Records != before.Records || copyAudit.LogicalSHA256 != before.LogicalSHA256 || copyAudit.Capacity != capacity {
		return out, ErrGateway
	}
	return AttemptCopyAudit{before, copyAudit}, nil
}
