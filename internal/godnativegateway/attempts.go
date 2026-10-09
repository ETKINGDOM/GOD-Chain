//go:build go1.25

package godnativegateway

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/ETKINGDOM/GOD-Chain/internal/godnative"
	bolt "go.etcd.io/bbolt"
)

const attemptFile = "native-attempts.db"
const attemptLimit = 1000
const attemptFileLimit = 4 << 20

// Larger durable capacity is opt-in through offline initialization or copying.
// The process cache remains bounded independently; disk hashes never expire.
// MaxAttemptCapacity bounds the candidate's retained disk inventory.
const MaxAttemptCapacity = 100000

func attemptCapacity(c Config) uint64 {
	if c.AttemptCapacity == 0 {
		return attemptLimit
	}
	return c.AttemptCapacity
}

func validAttemptCapacity(n uint64) bool {
	return n == 0 || n >= attemptLimit && n <= MaxAttemptCapacity
}

func attemptFileBudget(c Config) int64 {
	if attemptCapacity(c) == attemptLimit {
		return attemptFileLimit
	}
	return 32 << 20
}

var attemptMeta = []byte("meta")
var attemptHashes = []byte("blocked")

// Only nonsecret policy identity and hashes are retained. No signed wire,
// account, IP, timestamp, response, wallet or node data enters this database.
type attemptIdentity struct {
	Version           int    `json:"version"`
	ChainID           string `json:"chainId"`
	CompatibleChainID string `json:"compatibleChainId"`
	Gas               uint64 `json:"gas"`
	MinFeePerGas      string `json:"minFeePerGas"`
	Capacity          uint64 `json:"capacity,omitempty"`
}

func identity(c Config) []byte {
	b, _ := json.Marshal(attemptIdentity{2, c.ChainID, c.CompatibleChainID, c.Gas, c.MinFeePerGas, attemptCapacity(c)})
	return b
}

// Version one had an implicit capacity of 1,000. It may only be opened at that
// capacity; an offline copy writes version two without changing the source.
func legacyIdentity(c Config) []byte {
	b, _ := json.Marshal(attemptIdentity{1, c.ChainID, c.CompatibleChainID, c.Gas, c.MinFeePerGas, 0})
	return b
}

type attemptStore struct {
	db       *bolt.DB
	root     *os.Root
	home     string
	file     os.FileInfo
	closed   bool
	count    uint64
	capacity uint64
	maxFile  int64
	policy   []byte
	logical  string
}

// InitializeAttempts is an explicit offline bootstrap, never called by New or
// Serve. It only creates in an existing empty owner-only directory. Reviewed
// legacy uncertain hashes can be carried forward without their signed bytes.
// This is not a migration from, or reconciliation of, a running process's map.
func InitializeAttempts(c Config, blocked []string) error {
	if !validAttemptPolicy(c) || uint64(len(blocked)) > attemptCapacity(c) {
		return ErrGateway
	}
	seen := map[string]bool{}
	for _, hash := range blocked {
		if !lowerHash(hash, true) || seen[hash] {
			return ErrGateway
		}
		seen[hash] = true
	}
	s, err := openAttempts(c, true, blocked, nil)
	if err != nil {
		return ErrGateway
	}
	return s.close()
}

func validAttemptPolicy(c Config) bool {
	if !validConfig(c) || c.ValidatorOnly {
		return false
	}
	_, err := godnative.New(godnative.Policy{ChainID: c.ChainID, Gas: c.Gas, MinFeePerGas: c.MinFeePerGas})
	return err == nil
}

// from is an exclusively locked, audited source. The entire copy and its
// metadata enter the destination's FIRST initialization transaction together.
// There is no intermediate valid, empty destination and no source mutation.
func openAttempts(c Config, create bool, seed []string, from *attemptStore) (store *attemptStore, err error) {
	if !validAttemptCapacity(c.AttemptCapacity) || from != nil && (!create || len(seed) != 0 || from.count > attemptCapacity(c)) {
		return nil, ErrGateway
	}
	var db *bolt.DB
	var root *os.Root
	var handle *os.File
	keep := false
	defer func() {
		if recover() != nil {
			store, err = nil, ErrGateway
		}
		if !keep {
			if db != nil {
				_ = db.Close()
			} else if handle != nil {
				_ = handle.Close()
			}
			if root != nil {
				_ = root.Close()
			}
		}
	}()
	home := c.AttemptDirectory
	if !filepath.IsAbs(home) || filepath.Clean(home) != home || home == string(filepath.Separator) {
		return nil, ErrGateway
	}
	info, e := os.Lstat(home)
	if e != nil || !attemptOwner(info, true) {
		return nil, ErrGateway
	}
	root, e = os.OpenRoot(home)
	if e != nil {
		return nil, ErrGateway
	}
	actual, e := root.Stat(".")
	if e != nil || !attemptOwner(actual, true) || !os.SameFile(info, actual) {
		return nil, ErrGateway
	}
	dir, e := root.Open(".")
	if e != nil {
		return nil, ErrGateway
	}
	entries, e := dir.ReadDir(2)
	_ = dir.Close()
	if e != nil && e != io.EOF || create && len(entries) != 0 || !create && (len(entries) != 1 || entries[0].Name() != attemptFile) {
		return nil, ErrGateway
	}
	prior, e := root.Lstat(attemptFile)
	if create {
		if !os.IsNotExist(e) {
			return nil, ErrGateway
		}
	} else if e != nil || !attemptOwner(prior, false) || prior.Size() < 8192 || prior.Size() > attemptFileBudget(c) {
		return nil, ErrGateway
	}
	var opened os.FileInfo
	options := &bolt.Options{Timeout: 200 * time.Millisecond}
	options.OpenFile = func(path string, flags int, mode os.FileMode) (*os.File, error) {
		if path != filepath.Join(home, attemptFile) || mode.Perm() != 0600 {
			return nil, ErrGateway
		}
		if create {
			flags |= os.O_EXCL
		} else {
			flags &^= os.O_CREATE
		}
		f, e := root.OpenFile(attemptFile, flags|attemptNoFollow(), 0600)
		if e != nil {
			return nil, ErrGateway
		}
		stat, e := f.Stat()
		if e != nil || !attemptOwner(stat, false) || !create && !os.SameFile(prior, stat) {
			_ = f.Close()
			return nil, ErrGateway
		}
		opened = stat
		handle = f
		return f, nil
	}
	db, e = bolt.Open(filepath.Join(home, attemptFile), 0600, options)
	if e != nil {
		return nil, ErrGateway
	}
	db.AllocSize = 64 << 10
	s := &attemptStore{db: db, root: root, home: home, file: opened, capacity: attemptCapacity(c), maxFile: attemptFileBudget(c)}
	if create {
		e = db.Update(func(tx *bolt.Tx) error {
			m, e := tx.CreateBucket(attemptMeta)
			if e != nil {
				return ErrGateway
			}
			b, e := tx.CreateBucket(attemptHashes)
			if e != nil || m.Put([]byte("identity"), identity(c)) != nil {
				return ErrGateway
			}
			for _, hash := range seed {
				if b.Put([]byte(hash), []byte{1}) != nil {
					return ErrGateway
				}
			}
			if from != nil {
				return from.db.View(func(source *bolt.Tx) error {
					if !from.metadataOK(source) {
						return ErrGateway
					}
					var n uint64
					err := source.Bucket(attemptHashes).ForEach(func(k, v []byte) error {
						n++
						if n > s.capacity || !lowerHash(string(k), true) || !bytes.Equal(v, []byte{1}) {
							return ErrGateway
						}
						return b.Put(k, v)
					})
					if err != nil || n != from.count {
						return ErrGateway
					}
					return nil
				})
			}
			return nil
		})
		if e == nil {
			d, f := root.Open(".")
			if f != nil {
				e = ErrGateway
			} else {
				e = d.Sync()
				_ = d.Close()
			}
		}
	}
	if e != nil || !s.fileOK() {
		return nil, ErrGateway
	}
	h := sha256.New()
	_, _ = h.Write([]byte("GOD native attempt inventory v1\x00"))
	_, _ = h.Write(legacyIdentity(c))
	_, _ = h.Write([]byte{0})
	e = db.View(func(tx *bolt.Tx) error {
		count := 0
		if tx.ForEach(func(name []byte, _ *bolt.Bucket) error {
			count++
			if !bytes.Equal(name, attemptMeta) && !bytes.Equal(name, attemptHashes) {
				return ErrGateway
			}
			return nil
		}) != nil || count != 2 {
			return ErrGateway
		}
		m, b := tx.Bucket(attemptMeta), tx.Bucket(attemptHashes)
		if m == nil || b == nil || m.Stats().KeyN != 1 {
			return ErrGateway
		}
		policy := m.Get([]byte("identity"))
		if !bytes.Equal(policy, identity(c)) && !(s.capacity == attemptLimit && bytes.Equal(policy, legacyIdentity(c))) {
			return ErrGateway
		}
		s.policy = bytes.Clone(policy)
		return b.ForEach(func(k, v []byte) error {
			if s.count >= s.capacity || !lowerHash(string(k), true) || !bytes.Equal(v, []byte{1}) {
				return ErrGateway
			}
			s.count++
			_, _ = h.Write(k)
			_, _ = h.Write(v)
			return nil
		})
	})
	if e != nil {
		return nil, ErrGateway
	}
	s.logical = hex.EncodeToString(h.Sum(nil))
	keep = true
	return s, nil
}

func (s *attemptStore) fileOK() bool {
	if s == nil || s.closed || s.file == nil {
		return false
	}
	path, e := os.Lstat(s.home)
	d, f := s.root.Stat(".")
	file, g := s.root.Lstat(attemptFile)
	if e != nil || f != nil || g != nil || !attemptOwner(path, true) || !attemptOwner(d, true) || !os.SameFile(path, d) || !attemptOwner(file, false) || !os.SameFile(file, s.file) || file.Size() < 8192 || file.Size() > s.maxFile {
		return false
	}
	dir, e := s.root.Open(".")
	if e != nil {
		return false
	}
	entries, e := dir.ReadDir(2)
	_ = dir.Close()
	return (e == nil || e == io.EOF) && len(entries) == 1 && entries[0].Name() == attemptFile
}

func (s *attemptStore) metadataOK(tx *bolt.Tx) bool {
	m, b := tx.Bucket(attemptMeta), tx.Bucket(attemptHashes)
	return m != nil && b != nil && bytes.Equal(m.Get([]byte("identity")), s.policy)
}

// Membership is read from the locked disk store, not an expiring RAM cache.
// The Gateway serializes calls; offline consumers own the lock exclusively.
func (s *attemptStore) contains(hash string) (found bool, err error) {
	defer func() {
		if recover() != nil {
			found, err = false, ErrGateway
		}
	}()
	if !lowerHash(hash, true) || !s.fileOK() {
		return false, ErrGateway
	}
	err = s.db.View(func(tx *bolt.Tx) error {
		if !s.metadataOK(tx) {
			return ErrGateway
		}
		v := tx.Bucket(attemptHashes).Get([]byte(hash))
		if v != nil && !bytes.Equal(v, []byte{1}) || tx.Bucket(attemptHashes).Bucket([]byte(hash)) != nil {
			return ErrGateway
		}
		found = v != nil
		return nil
	})
	if err != nil || !s.fileOK() {
		return false, ErrGateway
	}
	return found, nil
}

// bbolt's default synchronous commit completes before node admission. Any
// failure is ambiguous storage state and makes the gateway permanently refuse
// further new submissions until an explicit operator recovery; no repair here.
func (s *attemptStore) change(hash string, remove bool) (err error) {
	defer func() {
		if recover() != nil {
			err = ErrGateway
		}
	}()
	if !lowerHash(hash, true) || !s.fileOK() {
		return ErrGateway
	}
	if s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(attemptHashes)
		if !s.metadataOK(tx) {
			return ErrGateway
		}
		if remove {
			if !bytes.Equal(b.Get([]byte(hash)), []byte{1}) {
				return ErrGateway
			}
			return b.Delete([]byte(hash))
		}
		if b.Get([]byte(hash)) != nil || b.Bucket([]byte(hash)) != nil || s.count >= s.capacity {
			return ErrGateway
		}
		return b.Put([]byte(hash), []byte{1})
	}) != nil || !s.fileOK() {
		return ErrGateway
	}
	if remove {
		s.count--
	} else {
		s.count++
	}
	return nil
}

func (s *attemptStore) close() error {
	if s == nil || s.closed {
		return nil
	}
	s.closed = true
	e, f := s.db.Close(), s.root.Close()
	if e != nil || f != nil {
		return ErrGateway
	}
	return nil
}
