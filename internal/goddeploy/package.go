//go:build go1.25

// Package goddeploy prepares private Linux host-acceptance archives only. It
// never installs services, activates, uploads or provisions operational keys.
package goddeploy

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"debug/buildinfo"
	"debug/elf"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"
)

var ErrPackage = errors.New("private synthetic deployment package rejected")

const lifecycleSHA = "e981cc0e1f296fff12ebb0246dac870b0f2f111aa0ba49927f2d42b605ad182a"
const maxBinary = 256 << 20
const maxText = 1 << 20
const maxArchive = maxBinary + 32*maxText

var sourceFiles = []string{"TESTNET.md", "COMPANION.md", "DEPLOYMENT.md", "WALLET.md", "EXPLORER.md", "THIRD_PARTY_NOTICES.md", "UPSTREAM.lock.json", "go.mod", "go.sum", "licenses/GodCometBFT-Apache-2.0.txt", "licenses/GodCometBFT-NOTICE.txt", "deploy/nginx-http.conf.template"}
var operationalText = regexp.MustCompile(`(?i)0x[0-9a-f]{40}\b|(?:god1|godvaloper1|godvalcons1)[023456789acdefghjklmnpqrstuvwxyz]{20,}|BEGIN [A-Z ]*PRIVATE KEY|(?:privateKey|apiKey|password|secret)\s*[:=]\s*["']?[A-Za-z0-9_-]{8,}`)

type Options struct{ BinaryPath, SourceDir, OutputPath, TargetArch, ExpectedBinarySHA256 string }
type Entry struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}
type Manifest struct {
	Version                      uint32  `json:"version"`
	Target                       string  `json:"target"`
	Synthetic                    bool    `json:"synthetic"`
	RealAssets                   bool    `json:"realAssets"`
	PrivateHostAcceptanceOnly    bool    `json:"privateHostAcceptanceOnly"`
	RuntimeVerified              bool    `json:"runtimeVerified"`
	BinaryRedistributionReviewed bool    `json:"binaryRedistributionReviewed"`
	LifecycleSHA256              string  `json:"lifecycleSha256"`
	Files                        []Entry `json:"files"`
}
type Report struct {
	Created                      bool   `json:"created"`
	Verified                     bool   `json:"verified"`
	Unpacked                     bool   `json:"unpacked"`
	DirectoryVerified            bool   `json:"directoryVerified"`
	ServicesInstalled            bool   `json:"servicesInstalled"`
	NodeStarted                  bool   `json:"nodeStarted"`
	Target                       string `json:"target"`
	Files                        int    `json:"files"`
	SHA256                       string `json:"sha256"`
	Synthetic                    bool   `json:"synthetic"`
	RealAssets                   bool   `json:"realAssets"`
	RuntimeVerified              bool   `json:"runtimeVerified"`
	BinaryRedistributionReviewed bool   `json:"binaryRedistributionReviewed"`
}

func digestOK(s string) bool {
	raw, err := hex.DecodeString(s)
	return err == nil && len(raw) == 32 && hex.EncodeToString(raw) == s
}
func hashReader(r io.Reader) (string, int64, error) {
	h := sha256.New()
	n, err := io.Copy(h, r)
	return hex.EncodeToString(h.Sum(nil)), n, err
}

func root(path string, private bool) (*os.Root, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) == string(filepath.Separator) || private && runtime.GOOS == "windows" {
		return nil, ErrPackage
	}
	s, err := os.Lstat(path)
	if err != nil || !s.IsDir() || s.Mode()&os.ModeSymlink != 0 || private && s.Mode().Perm()&0077 != 0 {
		return nil, ErrPackage
	}
	r, err := os.OpenRoot(path)
	if err != nil {
		return nil, ErrPackage
	}
	opened, err := r.Stat(".")
	if err != nil || !os.SameFile(s, opened) {
		_ = r.Close()
		return nil, ErrPackage
	}
	return r, nil
}
func regular(r *os.Root, name string, limit int64, private bool) (*os.File, int64, error) {
	s, err := r.Lstat(name)
	if err != nil || !s.Mode().IsRegular() || s.Size() < 1 || s.Size() > limit || private && s.Mode().Perm()&0077 != 0 {
		return nil, 0, ErrPackage
	}
	f, err := r.Open(name)
	if err != nil {
		return nil, 0, ErrPackage
	}
	a, err := f.Stat()
	if err != nil || !os.SameFile(s, a) {
		_ = f.Close()
		return nil, 0, ErrPackage
	}
	return f, s.Size(), nil
}

func buildOK(info *debug.BuildInfo, arch string) bool {
	if info == nil || info.GoVersion != "go1.26.8" || info.Path != "github.com/ETKINGDOM/GOD-Chain/cmd/godd" || info.Main.Path != "github.com/ETKINGDOM/GOD-Chain" || info.Main.Replace != nil {
		return false
	}
	settings := map[string]string{}
	for _, s := range info.Settings {
		if _, exists := settings[s.Key]; exists {
			return false
		}
		settings[s.Key] = s.Value
	}
	if settings["GOOS"] != "linux" || settings["GOARCH"] != arch || settings["CGO_ENABLED"] != "0" || settings["-trimpath"] != "true" || settings["-buildmode"] != "exe" || settings["-compiler"] != "gc" {
		return false
	}
	pins := map[string]string{"github.com/cosmos/evm": "v0.6.3", "github.com/cosmos/cosmos-sdk": "v0.53.6", "github.com/cometbft/cometbft": "v0.38.21"}
	for _, d := range info.Deps {
		version, exists := pins[d.Path]
		if !exists {
			continue
		}
		if d.Version != version {
			return false
		}
		if d.Path == "github.com/cometbft/cometbft" {
			if d.Replace == nil || !filepath.IsAbs(d.Replace.Path) || filepath.Base(d.Replace.Path) != "module-v0.38.21" || d.Replace.Version != "(devel)" {
				return false
			}
		} else if d.Replace != nil {
			return false
		}
		delete(pins, d.Path)
	}
	return len(pins) == 0
}

func textOK(data []byte) bool {
	return len(data) > 0 && len(data) <= maxText && !operationalText.Match(data)
}
func allNames() []string {
	return append([]string{"bin/godd", "README.md", "systemd/god-chain-testnet.service"}, sourceFiles...)
}

// Create pins a reviewed binary before output creation and hashes it again
// while archiving. Failure leaves any newly created partial file untrusted;
// existing files and all source/node workspaces are never replaced or removed.
func Create(o Options) (Report, error) {
	if (o.TargetArch != "amd64" && o.TargetArch != "arm64") || !digestOK(o.ExpectedBinarySHA256) || !filepath.IsAbs(o.BinaryPath) || !filepath.IsAbs(o.OutputPath) {
		return Report{}, ErrPackage
	}
	in, err := root(filepath.Dir(o.BinaryPath), false)
	if err != nil {
		return Report{}, ErrPackage
	}
	defer in.Close()
	binary, size, err := regular(in, filepath.Base(o.BinaryPath), maxBinary, false)
	if err != nil {
		return Report{}, ErrPackage
	}
	defer binary.Close()
	object, err := elf.NewFile(binary)
	if err != nil || object.Class != elf.ELFCLASS64 || object.Data != elf.ELFDATA2LSB || object.Type != elf.ET_EXEC || (o.TargetArch == "amd64" && object.Machine != elf.EM_X86_64) || (o.TargetArch == "arm64" && object.Machine != elf.EM_AARCH64) {
		return Report{}, ErrPackage
	}
	info, err := buildinfo.Read(binary)
	if err != nil || !buildOK(info, o.TargetArch) {
		return Report{}, ErrPackage
	}
	sha, n, err := hashReader(io.LimitReader(binary, maxBinary+1))
	if err != nil || n != size || sha != o.ExpectedBinarySHA256 {
		return Report{}, ErrPackage
	}
	source, err := root(o.SourceDir, false)
	if err != nil {
		return Report{}, ErrPackage
	}
	defer source.Close()
	texts := map[string][]byte{"README.md": []byte(packageReadme), "systemd/god-chain-testnet.service": []byte(serviceTemplate)}
	for _, name := range sourceFiles {
		f, _, err := regular(source, name, maxText, false)
		if err != nil {
			return Report{}, ErrPackage
		}
		data, err := io.ReadAll(io.LimitReader(f, maxText+1))
		closeErr := f.Close()
		if err != nil || closeErr != nil || !textOK(data) {
			return Report{}, ErrPackage
		}
		texts[name] = data
	}
	var lock struct {
		Lifecycle struct {
			SHA string `json:"patchedSHA256"`
		} `json:"consensusLifecycleBuild"`
	}
	if json.Unmarshal(texts["UPSTREAM.lock.json"], &lock) != nil || lock.Lifecycle.SHA != lifecycleSHA {
		return Report{}, ErrPackage
	}
	manifest := Manifest{Version: 1, Target: "linux/" + o.TargetArch, Synthetic: true, PrivateHostAcceptanceOnly: true, LifecycleSHA256: lifecycleSHA}
	for _, name := range allNames() {
		entry := Entry{Name: name}
		if name == "bin/godd" {
			entry.Size, entry.SHA256 = size, sha
		} else {
			data := texts[name]
			h := sha256.Sum256(data)
			entry.Size, entry.SHA256 = int64(len(data)), hex.EncodeToString(h[:])
		}
		manifest.Files = append(manifest.Files, entry)
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		return Report{}, ErrPackage
	}
	outRoot, err := root(filepath.Dir(o.OutputPath), true)
	if err != nil {
		return Report{}, ErrPackage
	}
	defer outRoot.Close()
	out, err := outRoot.OpenFile(filepath.Base(o.OutputPath), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return Report{}, ErrPackage
	}
	defer out.Close()
	hasher := sha256.New()
	gz, err := gzip.NewWriterLevel(io.MultiWriter(out, hasher), gzip.BestSpeed)
	if err != nil {
		return Report{}, ErrPackage
	}
	tw := tar.NewWriter(gz)
	write := func(name string, length int64, mode int64, reader io.Reader) error {
		if tw.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: length, Typeflag: tar.TypeReg, Format: tar.FormatUSTAR}) != nil {
			return ErrPackage
		}
		n, err := io.Copy(tw, reader)
		if err != nil || n != length {
			return ErrPackage
		}
		return nil
	}
	if write("manifest.json", int64(len(raw)), 0644, strings.NewReader(string(raw))) != nil {
		return Report{}, ErrPackage
	}
	for _, entry := range manifest.Files {
		if entry.Name != "bin/godd" {
			if write(entry.Name, entry.Size, 0644, strings.NewReader(string(texts[entry.Name]))) != nil {
				return Report{}, ErrPackage
			}
			continue
		}
		if _, err := binary.Seek(0, io.SeekStart); err != nil {
			return Report{}, ErrPackage
		}
		h := sha256.New()
		if write(entry.Name, size, 0755, io.TeeReader(io.LimitReader(binary, maxBinary+1), h)) != nil || hex.EncodeToString(h.Sum(nil)) != sha {
			return Report{}, ErrPackage
		}
	}
	if tw.Close() != nil || gz.Close() != nil || out.Sync() != nil || out.Close() != nil {
		return Report{}, ErrPackage
	}
	d, err := outRoot.Open(".")
	if err != nil {
		return Report{}, ErrPackage
	}
	defer d.Close()
	if d.Sync() != nil {
		return Report{}, ErrPackage
	}
	return Report{Created: true, Target: manifest.Target, Files: len(manifest.Files) + 1, SHA256: hex.EncodeToString(hasher.Sum(nil)), Synthetic: true}, nil
}

// Verify checks exact members, permissions, sizes, hashes and trailing bytes
// without extracting or executing any archive content. The supplied digest
// must come from a trusted review; a digest is not a publisher signature.
func Verify(path, expected string) (Report, error) {
	raw, err := reviewedArchive(path, expected)
	if err != nil {
		return Report{}, ErrPackage
	}
	return verifyArchive(raw, expected)
}

// The digest and parser see one bounded snapshot. Never reopen an archive
// after verification or seek a mutable file to extract different bytes.
func reviewedArchive(path, expected string) ([]byte, error) {
	if !digestOK(expected) || !filepath.IsAbs(path) {
		return nil, ErrPackage
	}
	r, err := root(filepath.Dir(path), true)
	if err != nil {
		return nil, ErrPackage
	}
	defer r.Close()
	f, size, err := regular(r, filepath.Base(path), maxArchive, true)
	if err != nil {
		return nil, ErrPackage
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, maxArchive+1))
	if err != nil || int64(len(raw)) != size || len(raw) > maxArchive {
		return nil, ErrPackage
	}
	h := sha256.Sum256(raw)
	if hex.EncodeToString(h[:]) != expected {
		return nil, ErrPackage
	}
	return raw, nil
}

func verifyArchive(snapshot []byte, expected string) (Report, error) {
	buffer := bufio.NewReader(bytes.NewReader(snapshot))
	gz, err := gzip.NewReader(buffer)
	if err != nil {
		return Report{}, ErrPackage
	}
	defer gz.Close()
	gz.Multistream(false)
	tr := tar.NewReader(gz)
	h, err := tr.Next()
	if err != nil || h.Name != "manifest.json" || !headerOK(h, maxText, 0644) {
		return Report{}, ErrPackage
	}
	raw, err := io.ReadAll(io.LimitReader(tr, maxText+1))
	if err != nil {
		return Report{}, ErrPackage
	}
	var m Manifest
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&m) != nil || decoder.Decode(new(any)) != io.EOF || m.Version != 1 || (m.Target != "linux/amd64" && m.Target != "linux/arm64") || !m.Synthetic || m.RealAssets || !m.PrivateHostAcceptanceOnly || m.RuntimeVerified || m.BinaryRedistributionReviewed || m.LifecycleSHA256 != lifecycleSHA || len(m.Files) != len(allNames()) {
		return Report{}, ErrPackage
	}
	canonical, err := json.Marshal(m)
	if err != nil || !bytes.Equal(canonical, raw) {
		return Report{}, ErrPackage
	}
	wanted := map[string]Entry{}
	for i, name := range allNames() {
		entry := m.Files[i]
		if entry.Name != name || !digestOK(entry.SHA256) || entry.Size < 1 {
			return Report{}, ErrPackage
		}
		wanted[name] = entry
	}
	seen := map[string]bool{}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Report{}, ErrPackage
		}
		entry, exists := wanted[h.Name]
		limit, mode := int64(maxText), int64(0644)
		if h.Name == "bin/godd" {
			limit, mode = maxBinary, 0755
		}
		if !exists || seen[h.Name] || !headerOK(h, limit, mode) || h.Size != entry.Size {
			return Report{}, ErrPackage
		}
		var reader io.Reader = tr
		if h.Name == "bin/godd" {
			head := make([]byte, 64)
			if _, err := io.ReadFull(tr, head); err != nil || !binaryHeaderOK(head, strings.TrimPrefix(m.Target, "linux/")) {
				return Report{}, ErrPackage
			}
			reader = io.MultiReader(bytes.NewReader(head), tr)
		}
		sha, n, err := hashReader(io.LimitReader(reader, limit+1))
		if err != nil || n != entry.Size || sha != entry.SHA256 {
			return Report{}, ErrPackage
		}
		seen[h.Name] = true
	}
	if len(seen) != len(wanted) {
		return Report{}, ErrPackage
	}
	if n, err := io.Copy(io.Discard, io.LimitReader(gz, 1)); err != nil || n != 0 {
		return Report{}, ErrPackage
	}
	if _, err := buffer.Peek(1); err != io.EOF {
		return Report{}, ErrPackage
	}
	return Report{Verified: true, Target: m.Target, Files: len(m.Files) + 1, SHA256: expected, Synthetic: true}, nil
}

func headerOK(h *tar.Header, limit, mode int64) bool {
	return h.Typeflag == tar.TypeReg && h.Mode == mode && h.Size > 0 && h.Size <= limit && h.Linkname == "" && h.Uid == 0 && h.Gid == 0 && h.Uname == "" && h.Gname == "" && len(h.PAXRecords) == 0
}

func binaryHeaderOK(h []byte, arch string) bool {
	if len(h) < 64 || !bytes.Equal(h[:4], []byte{0x7f, 'E', 'L', 'F'}) || h[4] != 2 || h[5] != 1 || h[6] != 1 || binary.LittleEndian.Uint16(h[16:18]) != 2 {
		return false
	}
	m := binary.LittleEndian.Uint16(h[18:20])
	return arch == "amd64" && m == 62 || arch == "arm64" && m == 183
}

const serviceTemplate = `[Unit]
Description=GOD Chain synthetic testnet node
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=@NODE_USER@
Group=@NODE_GROUP@
UMask=0077
ExecStart=@GODD_BINARY@ testnet start --home @NODE_HOME@ --allow-network
Restart=on-failure
RestartSec=10
KillSignal=SIGTERM
TimeoutStopSec=60
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=@NODE_HOME@
RestrictSUIDSGID=true
LimitNOFILE=4096

[Install]
WantedBy=multi-user.target
`

const packageReadme = `# Private GOD Chain host acceptance candidate

This archive is for separately authorized private Linux host testing only.
It does not install a service, contain operational configuration or initialize
a chain. It contains no wallets, node keys, genesis, endpoints or source history.
The participant companion is embedded in the node binary and binds loopback only.

Review DEPLOYMENT.md, TESTNET.md and COMPANION.md before use. Compare the archive
digest through a trusted channel and run the non-extracting godpack verifier.
The separately reviewed godpack unpack command can stage the fixed inventory
in a new owner-only directory. It never starts a node or installs a service.
Use check-unpacked for a read-only recheck; never use a failed partial directory.
Initialize each operator independently on its own machine; never copy validator
keys or signing progress from a developer cluster. Keep real assets and RH off.

Cross-compilation and packaging do not establish Linux runtime acceptance or
complete binary redistribution review. This is not a public binary release.
Dependency notices and existing licenses are retained; any public distribution
requires a separate complete license, source and release review.
`
