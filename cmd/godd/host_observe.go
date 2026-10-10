package main

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ETKINGDOM/GOD-Chain/internal/goddeploy"
)

func runHostObserve(args []string, out io.Writer) error {
	flags := flag.NewFlagSet("host-observe", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	values, seen := map[string]string{}, map[string]bool{}
	names := []string{"data-dir", "pids", "samples", "interval-ms", "budget-seconds", "minimum-free-bytes", "minimum-memory-bytes", "maximum-rss-bytes", "maximum-open-files"}
	for _, name := range names {
		flags.Func(name, "", func(value string) error {
			if seen[name] {
				return goddeploy.ErrHostObserve
			}
			seen[name], values[name] = true, value
			return nil
		})
	}
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return goddeploy.ErrHostObserve
	}
	for _, name := range names {
		if !seen[name] {
			return goddeploy.ErrHostObserve
		}
	}
	integers := map[string]uint64{}
	for _, name := range names[2:] {
		n, err := strconv.ParseUint(values[name], 10, 64)
		if err != nil || strconv.FormatUint(n, 10) != values[name] || n > 1<<50 {
			return goddeploy.ErrHostObserve
		}
		integers[name] = n
	}
	if integers["samples"] > 60 || integers["interval-ms"] > 5000 || integers["budget-seconds"] > 180 || len(values["pids"]) > 352 {
		return goddeploy.ErrHostObserve
	}
	var pids []int
	for _, value := range strings.Split(values["pids"], ",") {
		n, err := strconv.ParseUint(value, 10, 31)
		if err != nil || strconv.FormatUint(n, 10) != value {
			return goddeploy.ErrHostObserve
		}
		pids = append(pids, int(n))
	}
	o := goddeploy.HostObserveOptions{DataDir: values["data-dir"], PIDs: pids, Samples: int(integers["samples"]), Interval: time.Duration(integers["interval-ms"]) * time.Millisecond,
		Budget: time.Duration(integers["budget-seconds"]) * time.Second, MinimumFreeBytes: integers["minimum-free-bytes"], MinimumMemoryBytes: integers["minimum-memory-bytes"],
		MaximumRSSBytes: integers["maximum-rss-bytes"], MaximumOpenFiles: integers["maximum-open-files"]}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	encoder := json.NewEncoder(out)
	r, err := goddeploy.HostObserve(ctx, o, func(s goddeploy.HostObserveSample) error { return encoder.Encode(s) })
	if r.Reason == "configuration" {
		return goddeploy.ErrHostObserve
	}
	if encoder.Encode(r) != nil {
		return goddeploy.ErrHostObserve
	}
	return err
}
