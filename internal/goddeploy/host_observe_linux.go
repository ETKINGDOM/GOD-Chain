//go:build go1.25 && linux

package goddeploy

import (
	"context"
	"io"
	"os"
	"strconv"
	"syscall"

	"golang.org/x/sys/unix"
)

type linuxHostObservation struct {
	options   HostObserveOptions
	proc      *os.Root
	data      *os.Root
	directory *os.File
	processes map[int]*os.Root
}

func observationOwned(s os.FileInfo) bool {
	u, ok := s.Sys().(*syscall.Stat_t)
	return ok && s.IsDir() && s.Mode()&os.ModeSymlink == 0 && u.Uid == uint32(os.Geteuid())
}

func openHostObservation(o HostObserveOptions) (hostObservationSource, string, error) {
	if os.Geteuid() == 0 {
		return nil, "privileged-user", ErrHostObserve
	}
	s := &linuxHostObservation{options: o, processes: map[int]*os.Root{}}
	ok := false
	defer func() {
		if !ok {
			_ = s.close()
		}
	}()
	var err error
	s.data, err = root(o.DataDir, true)
	if err != nil {
		return nil, "private-directory", ErrHostObserve
	}
	s.directory, err = s.data.Open(".")
	if err != nil || !s.directoryOK() {
		return nil, "private-directory", ErrHostObserve
	}
	s.proc, err = os.OpenRoot("/proc") // Fixed kernel path; no configurable proc root.
	if err != nil {
		return nil, "procfs-unavailable", ErrHostObserve
	}
	probe, err := s.proc.Open(".")
	if err != nil {
		return nil, "procfs-unavailable", ErrHostObserve
	}
	var fs unix.Statfs_t
	err = unix.Fstatfs(int(probe.Fd()), &fs)
	closeErr := probe.Close()
	if err != nil || closeErr != nil || fs.Type != unix.PROC_SUPER_MAGIC {
		return nil, "procfs-unavailable", ErrHostObserve
	}
	for _, pid := range o.PIDs {
		r, err := s.proc.OpenRoot(strconv.Itoa(pid))
		if err != nil {
			return nil, "process-unavailable", ErrHostObserve
		}
		s.processes[pid] = r
		st, err := r.Stat(".")
		if err != nil || !observationOwned(st) {
			return nil, "process-owner", ErrHostObserve
		}
	}
	ok = true
	return s, "", nil
}

func (s *linuxHostObservation) close() error {
	failed := false
	for _, r := range s.processes {
		failed = r.Close() != nil || failed
	}
	if s.directory != nil {
		failed = s.directory.Close() != nil || failed
	}
	for _, r := range []*os.Root{s.data, s.proc} {
		if r != nil {
			failed = r.Close() != nil || failed
		}
	}
	if failed {
		return ErrHostObserve
	}
	return nil
}

func (s *linuxHostObservation) directoryOK() bool {
	if s.directory == nil {
		return false
	}
	listed, err := os.Lstat(s.options.DataDir)
	opened, e := s.directory.Stat()
	return err == nil && e == nil && observationOwned(listed) && observationOwned(opened) &&
		listed.Mode().Perm()&0077 == 0 && opened.Mode().Perm()&0077 == 0 && os.SameFile(listed, opened)
}

func observeKernelFile(ctx context.Context, root *os.Root, name string, limit int64) ([]byte, error) {
	if ctx.Err() != nil {
		return nil, ErrHostObserve
	}
	st, err := root.Lstat(name)
	if err != nil || !st.Mode().IsRegular() {
		return nil, ErrHostObserve
	}
	f, err := root.Open(name)
	if err != nil {
		return nil, ErrHostObserve
	}
	opened, err := f.Stat()
	if err != nil || !os.SameFile(st, opened) {
		_ = f.Close()
		return nil, ErrHostObserve
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	closed := f.Close()
	if err != nil || closed != nil || len(data) == 0 || int64(len(data)) > limit || ctx.Err() != nil {
		return nil, ErrHostObserve
	}
	return data, nil
}

func observeProcessFiles(ctx context.Context, root *os.Root, remaining uint64) (count uint64, resultErr error) {
	if ctx.Err() != nil {
		return 0, ErrHostObserve
	}
	listed, err := root.Lstat("fd")
	if err != nil || !listed.IsDir() || listed.Mode()&os.ModeSymlink != 0 {
		return 0, ErrHostObserve
	}
	f, err := root.Open("fd") // Count entries only. Never open/readlink an FD target.
	if err != nil {
		return 0, ErrHostObserve
	}
	defer func() {
		if f.Close() != nil {
			count, resultErr = 0, ErrHostObserve
		}
	}()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(listed, opened) {
		return 0, ErrHostObserve
	}
	for {
		if ctx.Err() != nil {
			return 0, ErrHostObserve
		}
		names, err := f.Readdirnames(128)
		for _, name := range names {
			if _, err := hostKernelUint(name); err != nil {
				return 0, ErrHostObserve
			}
			count++
			if count > remaining {
				return 0, ErrHostObserve
			}
		}
		if err == io.EOF {
			if ctx.Err() != nil {
				return 0, ErrHostObserve
			}
			return count, nil
		}
		if err != nil {
			return 0, ErrHostObserve
		}
	}
}

func (s *linuxHostObservation) snapshot(ctx context.Context) (hostObservationState, error) {
	var out hostObservationState
	if ctx.Err() != nil || !s.directoryOK() {
		return out, ErrHostObserve
	}
	m, unprivileged, err := readHostMetrics(s.directory)
	if err != nil || !unprivileged {
		return out, ErrHostObserve
	}
	out.disk = m.available
	raw, err := observeKernelFile(ctx, s.proc, "meminfo", 64<<10)
	if err != nil {
		return out, ErrHostObserve
	}
	out.totalMemory, out.memory, err = parseHostMemory(raw)
	if err != nil {
		return out, ErrHostObserve
	}
	out.processes = map[int]observedProcess{}
	var files uint64
	for _, pid := range s.options.PIDs {
		r := s.processes[pid]
		owner, err := observeKernelFile(ctx, r, "status", 64<<10)
		if err != nil || parseHostProcessOwner(owner, pid, os.Geteuid()) != nil {
			return out, ErrHostObserve
		}
		raw, err := observeKernelFile(ctx, r, "stat", 8<<10)
		if err != nil {
			return out, ErrHostObserve
		}
		p, err := parseHostProcess(raw, pid, uint64(os.Getpagesize()))
		if err != nil {
			return out, ErrHostObserve
		}
		p.files, err = observeProcessFiles(ctx, r, 65536-files)
		if err != nil {
			return out, ErrHostObserve
		}
		after, err := observeKernelFile(ctx, r, "stat", 8<<10)
		if err != nil {
			return out, ErrHostObserve
		}
		check, err := parseHostProcess(after, pid, uint64(os.Getpagesize()))
		st, statErr := r.Stat(".")
		if err != nil || statErr != nil || !observationOwned(st) || check.start != p.start || check.user < p.user || check.system < p.system {
			return out, ErrHostObserve
		}
		out.processes[pid] = p
		files += p.files
	}
	raw, err = observeKernelFile(ctx, s.proc, "stat", 1<<20)
	if err != nil {
		return out, ErrHostObserve
	}
	cpu, err := parseHostCPU(raw)
	if err != nil || !s.directoryOK() || ctx.Err() != nil {
		return out, ErrHostObserve
	}
	out.cpu, out.logicalCPUs, out.cpuIDs = cpu.values, cpu.count, cpu.ids
	return out, nil
}
