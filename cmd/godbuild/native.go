// The optional native-gateway build changes only two pinned logging imports.
// It retains the GodCometBFT worker-join overlay and never edits module caches,
// upgrades dependencies, changes service hardening or starts a runtime.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const nativeLogModule = "cosmossdk.io/log"
const nativeLogVersion = "v1.6.1"

var nativeLogPins = map[string]string{
	"logger.go": "271629d51894c18221c40fe33b1c8e98d88e38aaa61429d09b5de4e98800789f",
	"writer.go": "647921f71ace1464653c4ebf7dfd84fccdecb5dc8ddc912f14078306c8264c64",
}

func nativeDigest(raw []byte) string {
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:])
}

func patchNativeLog(file string, raw []byte) ([]byte, error) {
	pin, ok := nativeLogPins[file]
	if !ok || nativeDigest(raw) != pin || strings.Count(string(raw), `"github.com/bytedance/sonic"`) != 1 {
		return nil, errors.New("native logging checksum changed; review required")
	}
	text := string(raw)
	if file == "logger.go" {
		text = strings.Replace(text, "\n\t\"github.com/bytedance/sonic\"", "", 1)
	} else {
		text = strings.Replace(text, `"github.com/bytedance/sonic"`, `"encoding/json"`, 1)
	}
	text = strings.ReplaceAll(text, "sonic.Marshal(", "json.Marshal(")
	text = strings.ReplaceAll(text, "sonic.Unmarshal(", "json.Unmarshal(")
	if strings.Contains(text, "sonic") {
		return nil, errors.New("native logging patch context changed; review required")
	}
	return []byte(text), nil
}

func nativeDirectory(path string) error {
	if err := os.Mkdir(path, 0700); err != nil && !os.IsExist(err) {
		return err
	}
	s, err := os.Lstat(path)
	if err != nil || !s.IsDir() || s.Mode()&os.ModeSymlink != 0 {
		return errors.New("unsafe native build directory")
	}
	return nil
}

func nativeBuildFile(path string, raw []byte) error {
	if s, err := os.Lstat(path); err == nil {
		if !s.Mode().IsRegular() || s.Mode()&os.ModeSymlink != 0 {
			return errors.New("unsafe native compiler input")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	return writeBuildFile(path, raw)
}

func nativeMirror(source, dir string) (string, error) {
	// Reject nonregular/symlink source entries before copying the verified cache.
	if err := verifyMirror(source, source); err != nil {
		return "", err
	}
	mirror := filepath.Join(dir, "log-"+nativeLogVersion)
	if _, err := os.Lstat(mirror); err == nil {
		return mirror, verifyMirror(source, mirror)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	stage, err := os.MkdirTemp(dir, ".native-log-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(stage) // Only this helper's newly allocated private stage.
	if err := os.CopyFS(stage, os.DirFS(source)); err != nil {
		return "", err
	}
	if err := os.Rename(stage, mirror); err != nil {
		return "", err
	}
	return mirror, verifyMirror(source, mirror)
}

func prepareNative(goBinary, root, baseOverlay string) (string, error) {
	base := filepath.Join(root, "build", ".godcomet")
	if baseOverlay != filepath.Join(base, "overlay.json") {
		return "", errors.New("native build requires the checked lifecycle overlay")
	}
	raw, err := os.ReadFile(baseOverlay)
	var overlay struct{ Replace map[string]string }
	if err != nil || json.Unmarshal(raw, &overlay) != nil || len(overlay.Replace) != 1 {
		return "", errors.New("native lifecycle overlay rejected")
	}
	for from, to := range overlay.Replace {
		patched, e := os.ReadFile(to)
		if from != filepath.Join(base, "module-"+consensusVersion, "consensus", "reactor.go") || e != nil || nativeDigest(patched) != patchedReactorSHA256 {
			return "", errors.New("native lifecycle compiler input rejected")
		}
	}
	modfile := "-modfile=" + filepath.Join(base, "god.mod")
	cmd := exec.Command(goBinary, "list", "-mod=readonly", modfile, "-m", "-json", nativeLogModule)
	cmd.Dir = root
	raw, err = cmd.Output()
	var m moduleInfo
	if err != nil || json.Unmarshal(raw, &m) != nil || m.Path != nativeLogModule || m.Version != nativeLogVersion || m.Replace != nil || !filepath.IsAbs(m.Dir) {
		return "", errors.New("native logging module pin changed; review required")
	}
	verify := exec.Command(goBinary, "mod", "verify", modfile)
	verify.Dir = root
	if verify.Run() != nil {
		return "", errors.New("native module checksum verification failed")
	}
	for _, dir := range []string{filepath.Join(root, "build"), base, filepath.Join(root, "build", ".godnative")} {
		if err := nativeDirectory(dir); err != nil {
			return "", err
		}
	}
	dir := filepath.Join(root, "build", ".godnative")
	mirror, err := nativeMirror(m.Dir, dir)
	if err != nil {
		return "", err
	}
	for _, file := range []string{"logger.go", "writer.go"} {
		raw, err := os.ReadFile(filepath.Join(mirror, file))
		if err != nil {
			return "", err
		}
		patched, err := patchNativeLog(file, raw)
		if err != nil {
			return "", err
		}
		to := filepath.Join(dir, "stdlib-"+file)
		if err := nativeBuildFile(to, patched); err != nil {
			return "", err
		}
		overlay.Replace[filepath.Join(mirror, file)] = to
	}
	for _, file := range []string{"god.mod", "god.sum"} {
		raw, err := os.ReadFile(filepath.Join(base, file))
		if err != nil {
			return "", err
		}
		if file == "god.mod" {
			raw = append(raw, []byte("\n// Gateway-only standard JSON logging; original pin unchanged.\nreplace "+nativeLogModule+" => "+strconv.Quote(mirror)+"\n")...)
		}
		if err := nativeBuildFile(filepath.Join(dir, file), raw); err != nil {
			return "", err
		}
	}
	raw, err = json.Marshal(overlay)
	if err != nil {
		return "", err
	}
	out := filepath.Join(dir, "overlay.json")
	if err := nativeBuildFile(out, raw); err != nil {
		return "", err
	}
	cmd = exec.Command(goBinary, "list", "-mod=readonly", "-modfile="+filepath.Join(dir, "god.mod"), "-overlay="+out, "-deps", "./cmd/godnative")
	cmd.Dir = root
	raw, err = cmd.Output()
	if err != nil {
		return "", errors.New("native dependency graph unavailable")
	}
	for _, name := range bytes.Split(raw, []byte{'\n'}) {
		if bytes.HasPrefix(name, []byte("github.com/bytedance/sonic")) || bytes.HasPrefix(name, []byte("github.com/cloudwego/base64x")) {
			return "", errors.New("native executable-memory dependency rejected")
		}
	}
	return out, nil
}
