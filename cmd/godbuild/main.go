// godbuild prepares a checksum-bound Go compiler overlay. It never modifies
// dependency caches, changes pins, starts a node or supplies operational data.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const consensusModule = "github.com/cometbft/cometbft"
const consensusVersion = "v0.38.21"
const reactorSHA256 = "e52650fe4f52a907bd6d43af06c5eb8ad210cd952d4abc69f349b89ecc43fc21"
const patchedReactorSHA256 = "e981cc0e1f296fff12ebb0246dac870b0f2f111aa0ba49927f2d42b605ad182a"

// These minimal excerpts modify the Apache-2.0 licensed upstream reactor at
// the exact pinned version. Original source and its license remain in the Go
// module cache; generated modified source remains ignored build material.
var reactorEdits = []struct{ before, after string }{
	{
		"\tMetrics *Metrics\n}",
		"\tMetrics *Metrics\n\n\t// God lifecycle patch: seal admission before joining peer store readers.\n\tpeerRoutineMtx sync.Mutex\n\tpeerRoutines sync.WaitGroup\n\tpeerRoutinesStopping bool\n}",
	},
	{
		"func (conR *Reactor) OnStop() {\n\tconR.unsubscribeFromBroadcastEvents()",
		"func (conR *Reactor) OnStop() {\n\tconR.peerRoutineMtx.Lock()\n\tconR.peerRoutinesStopping = true\n\tconR.peerRoutineMtx.Unlock()\n\tconR.unsubscribeFromBroadcastEvents()",
	},
	{
		"\tif !conR.WaitSync() {\n\t\tconR.conS.Wait()\n\t}\n}\n\n// SwitchToConsensus",
		"\tif !conR.WaitSync() {\n\t\tconR.conS.Wait()\n\t}\n\tconR.peerRoutines.Wait()\n}\n\n// WaitPeerRoutines joins admitted peer workers after reactor shutdown.\n// Calling it while the reactor runs is not a shutdown request.\nfunc (conR *Reactor) WaitPeerRoutines() { conR.peerRoutines.Wait() }\n\n// SwitchToConsensus",
	},
	{
		"func (conR *Reactor) AddPeer(peer p2p.Peer) {\n\tif !conR.IsRunning() {",
		"func (conR *Reactor) AddPeer(peer p2p.Peer) {\n\tconR.peerRoutineMtx.Lock()\n\tdefer conR.peerRoutineMtx.Unlock()\n\tif conR.peerRoutinesStopping || !conR.IsRunning() {",
	},
	{
		"\t// Begin routines for this peer.\n\tgo conR.gossipDataRoutine(peer, peerState)\n\tgo conR.gossipVotesRoutine(peer, peerState)\n\tgo conR.queryMaj23Routine(peer, peerState)",
		"\t// Admission and Add happen under the same lock as the stop seal.\n\tconR.peerRoutines.Add(3)\n\tgo func() { defer conR.peerRoutines.Done(); conR.gossipDataRoutine(peer, peerState) }()\n\tgo func() { defer conR.peerRoutines.Done(); conR.gossipVotesRoutine(peer, peerState) }()\n\tgo func() { defer conR.peerRoutines.Done(); conR.queryMaj23Routine(peer, peerState) }()",
	},
}

func patchReactor(raw []byte) ([]byte, error) {
	if fmt.Sprintf("%x", sha256.Sum256(raw)) != reactorSHA256 {
		return nil, errors.New("GodCometBFT reactor checksum mismatch; review required")
	}
	text := string(raw)
	for _, edit := range reactorEdits {
		if strings.Count(text, edit.before) != 1 {
			return nil, errors.New("GodCometBFT patch context mismatch; review required")
		}
		text = strings.Replace(text, edit.before, edit.after, 1)
	}
	patched, err := format.Source([]byte(text))
	if err != nil || fmt.Sprintf("%x", sha256.Sum256(patched)) != patchedReactorSHA256 {
		return nil, errors.New("GodCometBFT patched checksum mismatch; review required")
	}
	return patched, nil
}

type moduleInfo struct {
	Path, Version, Dir string
	Replace            *moduleInfo
}

func pinnedModule(goBinary, root string) (moduleInfo, error) {
	cmd := exec.Command(goBinary, "list", "-mod=readonly", "-m", "-json", consensusModule)
	cmd.Dir = root
	raw, err := cmd.Output()
	if err != nil {
		return moduleInfo{}, errors.New("cannot locate pinned GodCometBFT module")
	}
	var m moduleInfo
	if json.Unmarshal(raw, &m) != nil || !validModule(m) {
		return moduleInfo{}, errors.New("GodCometBFT module pin or replacement changed; review required")
	}
	return m, nil
}

func validModule(m moduleInfo) bool {
	return m.Path == consensusModule && m.Version == consensusVersion && m.Replace == nil && filepath.IsAbs(m.Dir)
}

// Build files are replaced atomically, not in the module cache. Identical
// concurrent preparation is harmless; compiler input uses its content hash.
func writeBuildFile(path string, data []byte) error {
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, data) {
		return nil
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".godbuild-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err := os.Rename(f.Name(), path); err != nil {
		// A concurrent identical preparation can win the rename on Windows.
		if old, readErr := os.ReadFile(path); readErr == nil && bytes.Equal(old, data) {
			return nil
		}
		return err
	}
	return nil
}

// Go forbids overlays within GOMODCACHE. Build against an exact private copy,
// then overlay that copy. Nothing is vendored into public production source.
func verifyMirror(source, mirror string) error {
	files := make(map[string]bool)
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		files[relative] = true
		info, err := os.Lstat(filepath.Join(mirror, relative))
		if err != nil || entry.IsDir() != info.IsDir() || info.Mode()&os.ModeSymlink != 0 || entry.Type()&os.ModeSymlink != 0 {
			return errors.New("GodCometBFT build mirror differs from verified source")
		}
		if entry.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() || !entry.Type().IsRegular() {
			return errors.New("unsupported GodCometBFT source object")
		}
		original, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		copied, err := os.ReadFile(filepath.Join(mirror, relative))
		if err != nil || !bytes.Equal(original, copied) {
			return errors.New("GodCometBFT build mirror bytes differ")
		}
		return nil
	})
	if err != nil {
		return err
	}
	return filepath.WalkDir(mirror, func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(mirror, path)
		if err != nil || !files[relative] {
			return errors.New("unexpected GodCometBFT build mirror file")
		}
		return nil
	})
}

func mirrorModule(source, dir string) (string, error) {
	mirror := filepath.Join(dir, "module-"+consensusVersion)
	if _, err := os.Lstat(mirror); err == nil {
		return mirror, verifyMirror(source, mirror)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	staging, err := os.MkdirTemp(dir, ".module-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(staging) // Only this newly created private build stage.
	if err := os.CopyFS(staging, os.DirFS(source)); err != nil {
		return "", err
	}
	if err := os.Rename(staging, mirror); err != nil {
		// A simultaneous verified preparation may have installed it first.
		if check := verifyMirror(source, mirror); check != nil {
			return "", err
		}
	}
	return mirror, verifyMirror(source, mirror)
}

func prepare(goBinary, root string) (string, string, error) {
	m, err := pinnedModule(goBinary, root)
	if err != nil {
		return "", "", err
	}
	verify := exec.Command(goBinary, "mod", "verify")
	verify.Dir = root
	if err := verify.Run(); err != nil {
		return "", "", errors.New("module checksum verification failed")
	}
	source := filepath.Join(m.Dir, "consensus", "reactor.go")
	raw, err := os.ReadFile(source)
	if err != nil {
		return "", "", err
	}
	patched, err := patchReactor(raw)
	if err != nil {
		return "", "", err
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(patched))
	dir := filepath.Join(root, "build", ".godcomet")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", "", err
	}
	mirror, err := mirrorModule(m.Dir, dir)
	if err != nil {
		return "", "", err
	}
	output := filepath.Join(dir, "reactor-"+digest+".go")
	if err := writeBuildFile(output, patched); err != nil {
		return "", "", err
	}
	config, err := json.MarshalIndent(struct{ Replace map[string]string }{map[string]string{filepath.Join(mirror, "consensus", "reactor.go"): output}}, "", "  ")
	if err != nil {
		return "", "", err
	}
	overlay := filepath.Join(dir, "overlay.json")
	if err := writeBuildFile(overlay, append(config, '\n')); err != nil {
		return "", "", err
	}
	mod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", "", err
	}
	sum, err := os.ReadFile(filepath.Join(root, "go.sum"))
	if err != nil {
		return "", "", err
	}
	mod = append(mod, []byte("\n// Private checksum-verified GodCometBFT lifecycle build.\nreplace "+consensusModule+" => "+strconv.Quote(mirror)+"\n")...)
	if err := writeBuildFile(filepath.Join(dir, "god.mod"), mod); err != nil {
		return "", "", err
	}
	if err := writeBuildFile(filepath.Join(dir, "god.sum"), sum); err != nil {
		return "", "", err
	}
	return overlay, digest, nil
}

func main() {
	goBinary := flag.String("go", filepath.Join(runtime.GOROOT(), "bin", "go"), "Go compiler used to locate the pinned module")
	native := flag.Bool("native-gateway", false, "prepare the separate checksum-bound no-JIT native gateway build")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "expected only the optional -go flag")
		os.Exit(1)
	}
	root, err := os.Getwd()
	if err == nil {
		var overlay, digest string
		overlay, digest, err = prepare(*goBinary, root)
		if err == nil && *native {
			overlay, err = prepareNative(*goBinary, root, overlay)
		}
		if err == nil {
			_ = json.NewEncoder(os.Stdout).Encode(struct {
				Overlay, PatchedSHA256 string
				GatewayOnly            bool `json:"gatewayOnly,omitempty"`
			}{overlay, digest, *native})
			return
		}
	}
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
