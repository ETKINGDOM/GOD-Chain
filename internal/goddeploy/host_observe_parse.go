//go:build go1.25

package goddeploy

import (
	"strconv"
	"strings"
)

func hostKernelUint(s string) (uint64, error) {
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil || strconv.FormatUint(n, 10) != s {
		return 0, ErrHostObserve
	}
	return n, nil
}

type observedCPU struct {
	values [8]uint64
	ids    [64]uint64
	count  int
}

// Only eight nonduplicated CPU time columns enter the ratio. guest/guest_nice
// are already included in user/nice. CPU-list changes invalidate the interval.
func parseHostCPU(raw []byte) (observedCPU, error) {
	var out observedCPU
	if len(raw) == 0 || len(raw) > 1<<20 {
		return out, ErrHostObserve
	}
	lines := strings.Split(string(raw), "\n")
	first := strings.Fields(lines[0])
	if len(first) != 11 || first[0] != "cpu" {
		return out, ErrHostObserve
	}
	for i, value := range first[1:] {
		n, err := hostKernelUint(value)
		if err != nil {
			return observedCPU{}, ErrHostObserve
		}
		if i < 8 {
			out.values[i] = n
		}
	}
	for _, line := range lines[1:] {
		values := strings.Fields(line)
		if len(values) == 0 || !strings.HasPrefix(values[0], "cpu") {
			continue
		}
		if values[0] == "cpu" || len(values) != 11 {
			return observedCPU{}, ErrHostObserve
		}
		id, err := hostKernelUint(values[0][3:])
		if err != nil || id > 4095 || out.ids[id/64]&(uint64(1)<<(id%64)) != 0 {
			return observedCPU{}, ErrHostObserve
		}
		out.ids[id/64] |= uint64(1) << (id % 64)
		out.count++
		for _, value := range values[1:] {
			if _, err := hostKernelUint(value); err != nil {
				return observedCPU{}, ErrHostObserve
			}
		}
	}
	if out.count < 1 || out.count > 4096 {
		return observedCPU{}, ErrHostObserve
	}
	return out, nil
}

func parseHostMemory(raw []byte) (total, available uint64, err error) {
	if len(raw) == 0 || len(raw) > 64<<10 {
		return 0, 0, ErrHostObserve
	}
	seen := map[string]bool{}
	for _, line := range strings.Split(string(raw), "\n") {
		v := strings.Fields(line)
		if len(v) == 0 || v[0] != "MemTotal:" && v[0] != "MemAvailable:" {
			continue
		}
		if len(v) != 3 || v[2] != "kB" || seen[v[0]] {
			return 0, 0, ErrHostObserve
		}
		seen[v[0]] = true
		n, err := hostKernelUint(v[1])
		if err != nil {
			return 0, 0, ErrHostObserve
		}
		bytes, err := hostCapacity(n, 1024)
		if err != nil {
			return 0, 0, ErrHostObserve
		}
		if v[0] == "MemTotal:" {
			total = bytes
		} else {
			available = bytes
		}
	}
	if len(seen) != 2 || total == 0 || available > total {
		return 0, 0, ErrHostObserve
	}
	return total, available, nil
}

// comm can contain spaces and parentheses. It is ignored and never disclosed;
// splitting the entire stat string on spaces would corrupt field offsets.
func parseHostProcess(raw []byte, pid int, pageBytes uint64) (observedProcess, error) {
	var p observedProcess
	if len(raw) == 0 || len(raw) > 8<<10 || pid < 2 || !strings.HasPrefix(string(raw), strconv.Itoa(pid)+" (") {
		return p, ErrHostObserve
	}
	end := strings.LastIndex(string(raw), ") ")
	if end < len(strconv.Itoa(pid))+3 {
		return p, ErrHostObserve
	}
	values := strings.Fields(string(raw)[end+2:])
	if len(values) < 22 || len(values[0]) != 1 || !strings.Contains("RSDTtKWPI", values[0]) {
		return p, ErrHostObserve
	}
	for _, field := range []struct {
		index int
		out   *uint64
	}{{11, &p.user}, {12, &p.system}, {19, &p.start}, {21, &p.rss}} {
		n, err := hostKernelUint(values[field.index])
		if err != nil {
			return observedProcess{}, ErrHostObserve
		}
		*field.out = n
	}
	var err error
	p.rss, err = hostCapacity(p.rss, pageBytes)
	if err != nil || p.start == 0 {
		return observedProcess{}, ErrHostObserve
	}
	return p, nil
}

// Restrict to same-UID process leaders, avoiding duplicate accounting of a
// thread and its whole thread group. No Name/argv/environment is reported.
func parseHostProcessOwner(raw []byte, pid, uid int) error {
	if len(raw) == 0 || len(raw) > 64<<10 || uid < 1 {
		return ErrHostObserve
	}
	seen := map[string]bool{}
	for _, line := range strings.Split(string(raw), "\n") {
		v := strings.Fields(line)
		if len(v) == 0 || v[0] != "Pid:" && v[0] != "Tgid:" && v[0] != "Uid:" {
			continue
		}
		if seen[v[0]] {
			return ErrHostObserve
		}
		seen[v[0]] = true
		if v[0] != "Uid:" {
			if len(v) != 2 || v[1] != strconv.Itoa(pid) {
				return ErrHostObserve
			}
		} else {
			if len(v) != 5 {
				return ErrHostObserve
			}
			for _, value := range v[1:] {
				if value != strconv.Itoa(uid) {
					return ErrHostObserve
				}
			}
		}
	}
	if len(seen) != 3 {
		return ErrHostObserve
	}
	return nil
}
