//go:build go1.25

// godpack prepares/verifies private synthetic host-acceptance archives. It
// never executes archive content, contacts a network or installs services.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/ETKINGDOM/GOD-Chain/internal/goddeploy"
)

func run(args []string, out io.Writer) error {
	if len(args) == 0 || len(args) == 1 && args[0] == "help" {
		_, err := fmt.Fprintln(out, "GOD Chain private host acceptance packages\ncreate --binary <reviewed-linux-binary> --source <reviewed-module-directory> --output <new-private-archive> --arch <amd64|arm64> --expected-binary <reviewed-sha256>\nverify --archive <private-file> --expected-archive <reviewed-sha256>\nNo installation, extraction, upload, runtime certification or public binary release.")
		return err
	}
	if args[0] != "create" && args[0] != "verify" {
		return goddeploy.ErrPackage
	}
	flags := flag.NewFlagSet("godpack", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var binary, source, output, arch, expected, archive *string
	if args[0] == "create" {
		binary = flags.String("binary", "", "")
		source = flags.String("source", "", "")
		output = flags.String("output", "", "")
		arch = flags.String("arch", "", "")
		expected = flags.String("expected-binary", "", "")
	} else {
		archive = flags.String("archive", "", "")
		expected = flags.String("expected-archive", "", "")
	}
	if flags.Parse(args[1:]) != nil || flags.NArg() != 0 {
		return goddeploy.ErrPackage
	}
	var report goddeploy.Report
	var err error
	if args[0] == "create" {
		report, err = goddeploy.Create(goddeploy.Options{BinaryPath: *binary, SourceDir: *source, OutputPath: *output, TargetArch: *arch, ExpectedBinarySHA256: *expected})
	} else {
		report, err = goddeploy.Verify(*archive, *expected)
	}
	if err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(report)
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, goddeploy.ErrPackage)
		os.Exit(1)
	}
}
