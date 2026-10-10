//go:build go1.25

package godhistory

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	bolt "go.etcd.io/bbolt"
)

var metaBucket = []byte("meta")
var blocksBucket = []byte("blocks")
var recordsBucket = []byte("records")
var addressesBucket = []byte("addresses")
var hashesBucket = []byte("hashes")

type State struct {
	Version      int    `json:"version"`
	Config       Config `json:"config"`
	Height       int64  `json:"height"`
	Hash         string `json:"hash"`
	Time         string `json:"time"`
	Blocks       uint64 `json:"blocks"`
	Transactions uint64 `json:"transactions"`
	Unsupported  uint64 `json:"unsupportedTransactions"`
	Verified     bool   `json:"verified"`
	CheckedAt    string `json:"checkedAt"`
}
type Index struct {
	mu       sync.Mutex
	db       *bolt.DB
	root     *os.Root
	config   Config
	readOnly bool
}

// Open touches only history.db in an existing, otherwise empty owner-only
// directory. A different source/coverage policy cannot reuse a populated index.
// No automatic truncation, pruning, file repair or checkpoint reset is provided.
func Open(home string, c Config, readOnly bool) (index *Index, err error) {
	var db *bolt.DB
	defer func() {
		if recover() != nil {
			if db != nil {
				db.Close()
			}
			index = nil
			err = ErrHistory
		}
	}()
	if !c.valid() || !filepath.IsAbs(home) || filepath.Clean(home) != home || home == string(filepath.Separator) {
		return nil, ErrHistory
	}
	info, e := os.Lstat(home)
	if e != nil || !owner(info, true) {
		return nil, ErrHistory
	}
	root, e := os.OpenRoot(home)
	if e != nil {
		return nil, ErrHistory
	}
	keep := false
	defer func() {
		if !keep {
			root.Close()
		}
	}()
	actual, e := root.Stat(".")
	if e != nil || !os.SameFile(info, actual) || !owner(actual, true) {
		return nil, ErrHistory
	}
	directory, e := root.Open(".")
	if e != nil {
		return nil, ErrHistory
	}
	entries, e := directory.ReadDir(2)
	directory.Close()
	if e != nil && e != io.EOF || len(entries) > 1 || len(entries) == 1 && entries[0].Name() != "history.db" {
		return nil, ErrHistory
	}
	prior, e := root.Lstat("history.db")
	exists := e == nil
	if e != nil && !os.IsNotExist(e) || exists && (!owner(prior, false) || prior.Size() < 8192 || prior.Size() > maxFile) || !exists && readOnly {
		return nil, ErrHistory
	}
	options := &bolt.Options{Timeout: 200 * time.Millisecond, ReadOnly: readOnly}
	options.OpenFile = func(path string, flags int, mode os.FileMode) (*os.File, error) {
		if path != filepath.Join(home, "history.db") || mode.Perm() != 0600 {
			return nil, ErrHistory
		}
		if !exists {
			flags |= os.O_EXCL
		}
		f, e := root.OpenFile("history.db", flags|noFollow(), 0600)
		if e != nil {
			return nil, ErrHistory
		}
		stat, e := f.Stat()
		if e != nil || !owner(stat, false) || exists && !os.SameFile(prior, stat) {
			f.Close()
			return nil, ErrHistory
		}
		return f, nil
	}
	db, e = bolt.Open(filepath.Join(home, "history.db"), 0600, options)
	if e != nil {
		return nil, ErrHistory
	}
	db.AllocSize = 1 << 20
	x := &Index{db: db, root: root, config: c, readOnly: readOnly}
	if !exists {
		e = db.Update(func(tx *bolt.Tx) error {
			for _, name := range [][]byte{metaBucket, blocksBucket, recordsBucket, addressesBucket, hashesBucket} {
				if _, e := tx.CreateBucket(name); e != nil {
					return ErrHistory
				}
			}
			return putState(tx, State{Version: 1, Config: c})
		})
		if e == nil {
			d, f := root.Open(".")
			if f != nil {
				e = ErrHistory
			} else {
				e = d.Sync()
				d.Close()
			}
		}
	}
	if e == nil {
		e = db.View(func(tx *bolt.Tx) error { _, e := x.state(tx); return e })
	}
	if e != nil {
		db.Close()
		return nil, ErrHistory
	}
	keep = true
	return x, nil
}
func (x *Index) Close() error {
	x.mu.Lock()
	defer x.mu.Unlock()
	e := x.db.Close()
	f := x.root.Close()
	if e != nil || f != nil {
		return ErrHistory
	}
	return nil
}
func (x *Index) state(tx *bolt.Tx) (State, error) {
	var s State
	for _, name := range [][]byte{metaBucket, blocksBucket, recordsBucket, addressesBucket, hashesBucket} {
		if tx.Bucket(name) == nil {
			return s, ErrHistory
		}
	}
	if exact(tx.Bucket(metaBucket).Get([]byte("state")), &s) != nil || s.Version != 1 || s.Config != x.config || s.Blocks > s.Config.MaxBlocks || s.Transactions > s.Config.MaxTransactions || s.Unsupported > s.Transactions {
		return s, ErrHistory
	}
	if s.Verified && s.CheckedAt == "" {
		return s, ErrHistory
	}
	if s.CheckedAt != "" {
		if _, e := stamp(s.CheckedAt); e != nil {
			return s, e
		}
	}
	if s.Blocks == 0 {
		if s.Height != 0 || s.Hash != "" || s.Time != "" || s.Transactions != 0 || s.Unsupported != 0 {
			return s, ErrHistory
		}
		return s, nil
	}
	if s.Height < s.Config.FirstHeight || uint64(s.Height-s.Config.FirstHeight)+1 != s.Blocks || !hash(s.Hash) {
		return s, ErrHistory
	}
	b, e := storedBlock(tx, s.Height)
	if e != nil || b.Hash != s.Hash || b.Time != s.Time || b.Unsupported != s.Unsupported {
		return s, ErrHistory
	}
	return s, nil
}
func putState(tx *bolt.Tx, s State) error {
	raw, e := json.Marshal(s)
	if e != nil || tx.Bucket(metaBucket).Put([]byte("state"), raw) != nil {
		return ErrHistory
	}
	return nil
}
func heightKey(h int64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(h))
	return b
}
func tuple(h int64, i int) []byte {
	b := append(heightKey(h), make([]byte, 4)...)
	binary.BigEndian.PutUint32(b[8:], uint32(i))
	return b
}
func storedBlock(tx *bolt.Tx, h int64) (Block, error) {
	var b Block
	if exact(tx.Bucket(blocksBucket).Get(heightKey(h)), &b) != nil || b.Height != strconv.FormatInt(h, 10) || !hash(b.Hash) || b.Transactions < 0 || b.Transactions > BlockLimit || b.Unsupported > 100000 || b.Parent == "0x" && h != 1 || b.Parent != "0x" && !hash(b.Parent) {
		return b, ErrHistory
	}
	if _, e := stamp(b.Time); e != nil {
		return b, e
	}
	return b, nil
}
func (x *Index) Status() (s State, err error) {
	x.mu.Lock()
	defer x.mu.Unlock()
	defer func() {
		if recover() != nil {
			s = State{}
			err = ErrHistory
		}
	}()
	err = x.db.View(func(tx *bolt.Tx) error { var e error; s, e = x.state(tx); return e })
	if err != nil {
		return State{}, ErrHistory
	}
	return s, nil
}
func readPage(ctx context.Context, source Source, h int64, offset int) (blockPage, error) {
	var p blockPage
	if ctx == nil || ctx.Err() != nil {
		return p, ErrHistory
	}
	raw, e := source.Read(ctx, "god_block", []string{strconv.FormatInt(h, 10), strconv.Itoa(offset), "20"})
	if e != nil || ctx.Err() != nil || exact(raw, &p) != nil || p.Height != strconv.FormatInt(h, 10) || p.Offset != strconv.Itoa(offset) || !p.Synthetic || p.RealAssets || !hash(p.Hash) || p.Count < 0 || p.Count > BlockLimit || offset > p.Count || p.Parent == "0x" && h != 1 || p.Parent != "0x" && !hash(p.Parent) || p.Transactions == nil {
		return p, ErrHistory
	}
	if _, e := stamp(p.Time); e != nil {
		return p, e
	}
	count := p.Count - offset
	if count > PageLimit {
		count = PageLimit
	}
	if len(p.Transactions) != count {
		return p, ErrHistory
	}
	if offset+count < p.Count {
		if p.Next == nil || *p.Next != strconv.Itoa(offset+count) {
			return p, ErrHistory
		}
	} else if p.Next != nil {
		return p, ErrHistory
	}
	return p, nil
}
func readBlock(ctx context.Context, source Source, h int64) (Block, []projected, error) {
	first, e := readPage(ctx, source, h, 0)
	if e != nil {
		return Block{}, nil, e
	}
	b := Block{Height: first.Height, Hash: first.Hash, Parent: first.Parent, Time: first.Time, Transactions: first.Count}
	records := make([]projected, 0, first.Count)
	p := first
	seen := map[string]bool{}
	for {
		if p.Height != b.Height || p.Hash != b.Hash || p.Parent != b.Parent || p.Time != b.Time || p.Count != b.Transactions {
			return Block{}, nil, ErrHistory
		}
		for _, raw := range p.Transactions {
			r, e := project(raw, h, len(records))
			if e != nil || seen[r.Record.ConsensusHash] {
				return Block{}, nil, ErrHistory
			}
			seen[r.Record.ConsensusHash] = true
			records = append(records, r)
		}
		if p.Next == nil {
			break
		}
		offset, _ := strconv.Atoi(*p.Next)
		p, e = readPage(ctx, source, h, offset)
		if e != nil {
			return Block{}, nil, e
		}
	}
	return b, records, nil
}

type SyncReport struct {
	From           string `json:"from"`
	IndexedThrough string `json:"indexedThrough"`
	ObservedTip    string `json:"observedTip"`
	Blocks         int    `json:"blocks"`
	CaughtUp       bool   `json:"caughtUp"`
	Synthetic      bool   `json:"synthetic"`
	RealAssets     bool   `json:"realAssets"`
}

// Sync performs at most limit (1..128) new blocks, using <=7 pages per block.
// A complete block, its address rows, deduplication keys and checkpoint commit
// atomically with durable DB sync. Errors retain only previously complete blocks.
// Later calls recheck the checkpoint hash; regression/forks/pruning never reset it.
func (x *Index) Sync(ctx context.Context, source Source, limit int) (report SyncReport, err error) {
	x.mu.Lock()
	defer x.mu.Unlock()
	return x.syncLocked(ctx, source, limit, 0, "", nil)
}

type backfillAnchor struct {
	Hash, Parent, Time string
	Count              int
}

// A nonzero through bound is used only by explicit fixed-target backfill.
// Ordinary Sync retains its existing append-to-observed-tip semantics.
func (x *Index) syncLocked(ctx context.Context, source Source, limit int, through int64, targetHash string, anchor *backfillAnchor) (report SyncReport, err error) {
	defer func() {
		if recover() != nil {
			report = SyncReport{}
			err = ErrHistory
		}
	}()
	if ctx == nil || ctx.Err() != nil || x.readOnly || source == nil || limit < 1 || limit > BatchLimit || through < 0 || through > 0 && !hash(targetHash) {
		return report, ErrHistory
	}
	var s State
	if x.db.View(func(tx *bolt.Tx) error { var e error; s, e = x.state(tx); return e }) != nil {
		return report, ErrHistory
	}
	if through > 0 && s.Height > through {
		return report, ErrHistory
	}
	// An interrupted or rejected refresh must not leave the earlier verified
	// page readable as though the latest source check had succeeded.
	s.Verified = false
	if x.db.Update(func(tx *bolt.Tx) error { return putState(tx, s) }) != nil {
		return report, ErrHistory
	}
	raw, e := source.Read(ctx, "god_network", []string{})
	n, tip, f := parseNetwork(raw, x.config)
	if e != nil || ctx.Err() != nil || f != nil || tip < s.Height || tip < x.config.FirstHeight || through > tip {
		return report, ErrHistory
	}
	var target blockPage
	if through > 0 {
		target, e = readPage(ctx, source, through, 0)
		targetTime, targetTimeErr := stamp(target.Time)
		tipTime, _ := stamp(n.Commit.Time)
		if e != nil || target.Hash != targetHash || targetTimeErr != nil || targetTime.After(tipTime) {
			return report, ErrHistory
		}
		if anchor != nil {
			selected := backfillAnchor{target.Hash, target.Parent, target.Time, target.Count}
			if anchor.Hash != "" && *anchor != selected {
				return report, ErrHistory
			}
			*anchor = selected
		}
	}
	if s.Height > 0 {
		p, e := readPage(ctx, source, s.Height, 0)
		var b Block
		local := x.db.View(func(tx *bolt.Tx) error { var e error; b, e = storedBlock(tx, s.Height); return e })
		if e != nil || local != nil || p.Hash != s.Hash || p.Time != s.Time || p.Count != b.Transactions || p.Parent != b.Parent {
			return report, ErrHistory
		}
		if s.Height == through && (p.Hash != target.Hash || p.Time != target.Time || p.Count != target.Count || p.Parent != target.Parent) {
			return report, ErrHistory
		}
	}
	start := x.config.FirstHeight
	if s.Height > 0 {
		if s.Height == 9223372036854775807 {
			return report, ErrHistory
		}
		start = s.Height + 1
	}
	report = SyncReport{From: strconv.FormatInt(x.config.FirstHeight, 10), IndexedThrough: strconv.FormatInt(s.Height, 10), ObservedTip: strconv.FormatInt(tip, 10), Synthetic: true}
	end := tip
	if through > 0 && through < end {
		end = through
	}
	for h := start; h <= end && report.Blocks < limit; h++ {
		if ctx.Err() != nil {
			return report, ErrHistory
		}
		b, records, e := readBlock(ctx, source, h)
		if e != nil {
			return report, ErrHistory
		}
		if h == through && (b.Hash != target.Hash || b.Time != target.Time || b.Parent != target.Parent || b.Transactions != target.Count) {
			return report, ErrHistory
		}
		blockTime, e := stamp(b.Time)
		tipTime, _ := stamp(n.Commit.Time)
		if e != nil || blockTime.After(tipTime) || s.Height > 0 && (b.Parent != s.Hash || func() bool { previous, _ := stamp(s.Time); return blockTime.Before(previous) }()) {
			return report, ErrHistory
		}
		if s.Blocks >= s.Config.MaxBlocks || uint64(len(records)) > s.Config.MaxTransactions-s.Transactions {
			return report, ErrHistory
		}
		// Deliberately conservative COW headroom, not a filesystem quota or
		// measured capacity promise. Dense blocks may require a policy review.
		rows := 2 + len(records)*2
		for _, p := range records {
			rows += len(p.Participants)
		}
		headroom := int64(4<<20) + int64(rows)*(512<<10)
		e = x.db.Update(func(tx *bolt.Tx) error {
			current, e := x.state(tx)
			if e != nil || ctx.Err() != nil || current != s || tx.Size()+headroom > maxFile {
				return ErrHistory
			}
			// Conservative remaining-space guard. Allocation stays in 1 MiB
			// increments; no implicit pruning or unbounded record collection.
			info, e := x.root.Stat("history.db")
			if e != nil || info.Size()+headroom > maxFile {
				return ErrHistory
			}
			for i, p := range records {
				key := tuple(h, i)
				value, e := json.Marshal(p.Record)
				if e != nil || len(value) > 512 || len(p.Participants) > 128 {
					return ErrHistory
				}
				if tx.Bucket(hashesBucket).Get([]byte(p.Record.ConsensusHash)) != nil || tx.Bucket(recordsBucket).Get(key) != nil {
					return ErrHistory
				}
				if tx.Bucket(recordsBucket).Put(key, value) != nil || tx.Bucket(hashesBucket).Put([]byte(p.Record.ConsensusHash), key) != nil {
					return ErrHistory
				}
				for address, role := range p.Participants {
					if len(address) != 20 || role < 1 || role > 3 || tx.Bucket(addressesBucket).Put(append([]byte(address), key...), []byte{role}) != nil {
						return ErrHistory
					}
				}
				if p.Unsupported {
					current.Unsupported++
				}
			}
			b.Unsupported = current.Unsupported
			value, e := json.Marshal(b)
			if e != nil || tx.Bucket(blocksBucket).Put(heightKey(h), value) != nil {
				return ErrHistory
			}
			current.Height, current.Hash, current.Time = h, b.Hash, b.Time
			current.Blocks++
			current.Transactions += uint64(len(records))
			return putState(tx, current)
		})
		if e != nil {
			return report, ErrHistory
		}
		s.Height, s.Hash, s.Time = h, b.Hash, b.Time
		s.Blocks++
		s.Transactions += uint64(len(records))
		for _, p := range records {
			if p.Unsupported {
				s.Unsupported++
			}
		}
		report.Blocks++
		report.IndexedThrough = strconv.FormatInt(h, 10)
		if h == 9223372036854775807 {
			break
		}
	}
	lastRaw, e := source.Read(ctx, "god_network", []string{})
	last, lastTip, f := parseNetwork(lastRaw, x.config)
	if e != nil || ctx.Err() != nil || f != nil || lastTip < tip || lastTip == tip && last.Commit != n.Commit {
		return report, ErrHistory
	}
	firstTime, _ := stamp(n.Commit.Time)
	lastTime, _ := stamp(last.Commit.Time)
	if lastTime.Before(firstTime) {
		return report, ErrHistory
	}
	// Confirm the just-written anchor after the bounded run. A failed check
	// reports unavailability; it never erases already committed index records.
	if s.Height > 0 {
		p, e := readPage(ctx, source, s.Height, 0)
		var b Block
		local := x.db.View(func(tx *bolt.Tx) error { var e error; b, e = storedBlock(tx, s.Height); return e })
		if e != nil || local != nil || p.Hash != s.Hash || p.Time != s.Time || p.Count != b.Transactions || p.Parent != b.Parent {
			return report, ErrHistory
		}
	}
	if through > 0 {
		p, e := readPage(ctx, source, through, 0)
		if e != nil || p.Hash != target.Hash || p.Time != target.Time || p.Parent != target.Parent || p.Count != target.Count {
			return report, ErrHistory
		}
	}
	s.Verified = true
	s.CheckedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if x.db.Update(func(tx *bolt.Tx) error {
		if ctx.Err() != nil {
			return ErrHistory
		}
		return putState(tx, s)
	}) != nil {
		return report, ErrHistory
	}
	report.ObservedTip = strconv.FormatInt(lastTip, 10)
	report.CaughtUp = s.Height == lastTip
	return report, nil
}

type Cursor struct {
	SnapshotHeight string `json:"snapshotHeight"`
	SnapshotHash   string `json:"snapshotHash"`
	BeforeHeight   string `json:"beforeHeight"`
	BeforeIndex    string `json:"beforeIndex"`
}
type Activity struct {
	Record    Record `json:"transaction"`
	Direction string `json:"direction"`
}
type Page struct {
	From                    string     `json:"from"`
	Through                 string     `json:"through"`
	SnapshotHash            string     `json:"snapshotHash"`
	CheckedAt               string     `json:"checkedAt"`
	UnsupportedTransactions string     `json:"unsupportedTransactions"`
	Limit                   int        `json:"limit"`
	Next                    *Cursor    `json:"next"`
	Items                   []Activity `json:"items"`
	Synthetic               bool       `json:"synthetic"`
	RealAssets              bool       `json:"realAssets"`
}

// Page performs a descending prefix seek with at most twenty matches and one
// look-ahead. Cursors pin a stored anchor; append-only growth cannot mix pages.
// Empty results mean no supported sender/recipient match within this coverage,
// not no account activity, no internal transfer or no NFT ownership history.
func (x *Index) Page(address string, cursor *Cursor) (page Page, err error) {
	x.mu.Lock()
	defer x.mu.Unlock()
	defer func() {
		if recover() != nil {
			page = Page{}
			err = ErrHistory
		}
	}()
	raw, e := account(address)
	if e != nil {
		return page, ErrHistory
	}
	err = x.db.View(func(tx *bolt.Tx) error {
		s, e := x.state(tx)
		if e != nil || s.Height == 0 || !s.Verified {
			return ErrHistory
		}
		anchor := s.Height
		anchorHash := s.Hash
		before := tuple(anchor, BlockLimit)
		if cursor != nil {
			anchor, e = integer(cursor.SnapshotHeight, x.config.FirstHeight, s.Height)
			if e != nil || !hash(cursor.SnapshotHash) {
				return ErrHistory
			}
			b, e := storedBlock(tx, anchor)
			if e != nil || b.Hash != cursor.SnapshotHash {
				return ErrHistory
			}
			anchorHash = b.Hash
			h, e := integer(cursor.BeforeHeight, x.config.FirstHeight, anchor)
			if e != nil {
				return ErrHistory
			}
			i, e := integer(cursor.BeforeIndex, 0, BlockLimit-1)
			if e != nil {
				return ErrHistory
			}
			before = tuple(h, int(i))
			boundary := append(append([]byte{}, raw...), before...)
			role := tx.Bucket(addressesBucket).Get(boundary)
			if len(role) != 1 || role[0] < 1 || role[0] > 3 {
				return ErrHistory
			}
		}
		anchorBlock, e := storedBlock(tx, anchor)
		if e != nil || anchorBlock.Unsupported > s.Unsupported {
			return ErrHistory
		}
		page = Page{From: strconv.FormatInt(x.config.FirstHeight, 10), Through: strconv.FormatInt(anchor, 10), SnapshotHash: anchorHash, CheckedAt: s.CheckedAt, UnsupportedTransactions: strconv.FormatUint(anchorBlock.Unsupported, 10), Limit: PageLimit, Items: []Activity{}, Synthetic: true}
		key := append(append([]byte{}, raw...), before...)
		c := tx.Bucket(addressesBucket).Cursor()
		k, v := c.Seek(key)
		if k == nil {
			k, v = c.Last()
		} else {
			k, v = c.Prev()
		}
		for ; k != nil && bytes.HasPrefix(k, raw); k, v = c.Prev() {
			if len(k) != 32 || len(v) != 1 || v[0] < 1 || v[0] > 3 {
				return ErrHistory
			}
			h := int64(binary.BigEndian.Uint64(k[20:28]))
			i := int(binary.BigEndian.Uint32(k[28:]))
			if h < x.config.FirstHeight || h > anchor || i >= BlockLimit {
				return ErrHistory
			}
			var record Record
			if exact(tx.Bucket(recordsBucket).Get(k[20:]), &record) != nil || record.Height != strconv.FormatInt(h, 10) || record.Index != strconv.Itoa(i) || !hash(record.ConsensusHash) || record.Kind != "native" && record.Kind != "ethereum" || record.Kind == "ethereum" && !hash(record.CompatibleHash) || record.Kind == "native" && record.CompatibleHash != "" || record.Execution != "sdk-succeeded" && record.Execution != "sdk-failed" && record.Execution != "succeeded" && record.Execution != "failed" {
				return ErrHistory
			}
			if record.Kind == "native" && record.Execution != "sdk-succeeded" && record.Execution != "sdk-failed" || record.Kind == "ethereum" && record.Execution == "sdk-succeeded" {
				return ErrHistory
			}
			b, e := storedBlock(tx, h)
			if e != nil || i >= b.Transactions || !bytes.Equal(tx.Bucket(hashesBucket).Get([]byte(record.ConsensusHash)), k[20:]) {
				return ErrHistory
			}
			if len(page.Items) == PageLimit {
				last := page.Items[PageLimit-1].Record
				page.Next = &Cursor{page.Through, page.SnapshotHash, last.Height, last.Index}
				break
			}
			direction := map[byte]string{1: "outgoing", 2: "incoming", 3: "self"}[v[0]]
			page.Items = append(page.Items, Activity{record, direction})
		}
		return nil
	})
	if err != nil {
		return Page{}, ErrHistory
	}
	return page, nil
}
