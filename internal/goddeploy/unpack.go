//go:build go1.25

package goddeploy

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
)

// Unpack validates the full archive before creating a new private directory.
// It writes only the fixed package inventory, never runs a binary or installs
// services, and never overwrites/removes an existing file or node workspace.
// An interrupted write leaves private, untrusted material for manual review.
func Unpack(archive, expected, destination string) (Report, error) {
	if !unpackPathOK(destination) {
		return Report{}, ErrPackage
	}
	snapshot, err := reviewedArchive(archive, expected)
	if err != nil {
		return Report{}, ErrPackage
	}
	report, err := verifyArchive(snapshot, expected)
	if err != nil {
		return Report{}, ErrPackage
	}
	return unpackSnapshot(snapshot, destination, report)
}

func unpackPathOK(destination string) bool {
	return (runtime.GOOS == "linux" || runtime.GOOS == "darwin") && filepath.IsAbs(destination) &&
		filepath.Clean(destination) == destination && destination != string(filepath.Separator)
}

func unpackSnapshot(snapshot []byte, destination string, report Report) (Report, error) {
	parent, err := root(filepath.Dir(destination), true)
	if err != nil {
		return Report{}, ErrPackage
	}
	defer parent.Close()
	name := filepath.Base(destination)
	if parent.Mkdir(name, 0700) != nil {
		return Report{}, ErrPackage
	}
	staged, err := parent.OpenRoot(name)
	if err != nil {
		return Report{}, ErrPackage
	}
	defer staged.Close()
	listed, err := parent.Lstat(name)
	opened, statErr := staged.Stat(".")
	if err != nil || statErr != nil || listed.Mode() != os.ModeDir|0700 || !os.SameFile(listed, opened) {
		return Report{}, ErrPackage
	}

	gz, err := gzip.NewReader(bytes.NewReader(snapshot))
	if err != nil {
		return Report{}, ErrPackage
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	directories := map[string]bool{".": true}
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Report{}, ErrPackage
		}
		// Names come from the already verified immutable fixed inventory.
		dir := path.Dir(header.Name)
		if staged.MkdirAll(dir, 0700) != nil {
			return Report{}, ErrPackage
		}
		for d := dir; d != "."; d = path.Dir(d) {
			directories[d] = true
		}
		mode := fs.FileMode(0600)
		if header.Name == "bin/godd" {
			mode = 0700
		}
		file, err := staged.OpenFile(header.Name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if err != nil {
			return Report{}, ErrPackage
		}
		n, writeErr := io.Copy(file, tr)
		syncErr := file.Sync()
		closeErr := file.Close()
		if writeErr != nil || syncErr != nil || closeErr != nil || n != header.Size {
			return Report{}, ErrPackage
		}
	}
	// Verify actual bytes and inventory before returning any success report.
	if checkUnpackedSnapshot(snapshot, staged) != nil {
		return Report{}, ErrPackage
	}
	ordered := make([]string, 0, len(directories))
	for dir := range directories {
		ordered = append(ordered, dir)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(ordered)))
	for _, dir := range ordered {
		if syncDirectory(staged, dir) != nil {
			return Report{}, ErrPackage
		}
	}
	if syncDirectory(parent, ".") != nil {
		return Report{}, ErrPackage
	}
	report.Unpacked, report.DirectoryVerified = true, true
	return report, nil
}

func syncDirectory(root *os.Root, name string) error {
	f, err := root.Open(name)
	if err != nil {
		return ErrPackage
	}
	err = f.Sync()
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		return ErrPackage
	}
	return nil
}

// CheckUnpacked rechecks a private staging directory against a trusted exact
// archive snapshot, not a checksum invented from the installed files. It has
// no writes, repairs, startup, signing or network side effects.
func CheckUnpacked(archive, expected, destination string) (Report, error) {
	if !unpackPathOK(destination) {
		return Report{}, ErrPackage
	}
	snapshot, err := reviewedArchive(archive, expected)
	if err != nil {
		return Report{}, ErrPackage
	}
	report, err := verifyArchive(snapshot, expected)
	if err != nil {
		return Report{}, ErrPackage
	}
	dir, err := root(destination, true)
	if err != nil {
		return Report{}, ErrPackage
	}
	defer dir.Close()
	if checkUnpackedSnapshot(snapshot, dir) != nil {
		return Report{}, ErrPackage
	}
	report.DirectoryVerified = true
	return report, nil
}

func checkUnpackedSnapshot(snapshot []byte, destination *os.Root) error {
	top, err := destination.Stat(".")
	if err != nil || top.Mode() != os.ModeDir|0700 {
		return ErrPackage
	}
	gz, err := gzip.NewReader(bytes.NewReader(snapshot))
	if err != nil {
		return ErrPackage
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	files := map[string]bool{}
	directories := map[string]bool{".": true}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return ErrPackage
		}
		mode := fs.FileMode(0600)
		if h.Name == "bin/godd" {
			mode = 0700
		}
		info, err := destination.Lstat(h.Name)
		if err != nil || info.Mode() != mode || info.Size() != h.Size {
			return ErrPackage
		}
		file, _, err := regular(destination, h.Name, h.Size, true)
		if err != nil {
			return ErrPackage
		}
		diskHash, diskSize, readErr := hashReader(io.LimitReader(file, h.Size+1))
		closeErr := file.Close()
		archiveHash, archiveSize, archiveErr := hashReader(tr)
		if readErr != nil || closeErr != nil || archiveErr != nil || diskSize != h.Size || archiveSize != h.Size || diskHash != archiveHash {
			return ErrPackage
		}
		files[h.Name] = true
		for d := path.Dir(h.Name); d != "."; d = path.Dir(d) {
			directories[d] = true
		}
	}
	return fs.WalkDir(destination.FS(), ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return ErrPackage
		}
		info, err := entry.Info()
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return ErrPackage
		}
		if entry.IsDir() {
			if !directories[name] || info.Mode() != os.ModeDir|0700 {
				return ErrPackage
			}
		} else if !files[name] || !info.Mode().IsRegular() {
			return ErrPackage
		}
		return nil
	})
}
