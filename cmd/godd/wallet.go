package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/ETKINGDOM/GOD-Chain/internal/godtestnet"
	"golang.org/x/term"
)

// No password argument, environment variable, file flag, pipe or browser input.
// The private terminal is separate from stdout's redacted machine reports.
func runWallet(args []string, out io.Writer) error {
	if len(args) == 0 || len(args) == 1 && args[0] == "help" {
		_, err := fmt.Fprintln(out, "GOD Chain synthetic encrypted test wallet\nCommands: create, address, sign, sign-registration\nAll commands require --wallet <private-file> --bundle <private-file> --expected-bundle <reviewed-digest>. Address and signing require --output <new-private-file>; signing also requires --request <reviewed-private-file>.\nSign-registration is a separate offline operator action; it does not expand browser/gateway signing permissions. Passwords and signing approval are read only from a private interactive terminal. No real assets, key import, mnemonic recovery, browser signer or automatic submission.")
		return err
	}
	command := args[0]
	if command != "create" && command != "address" && command != "sign" && command != "sign-registration" {
		return godtestnet.ErrWallet
	}
	flags := flag.NewFlagSet("wallet", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	wallet := flags.String("wallet", "", "")
	bundle := flags.String("bundle", "", "")
	pin := flags.String("expected-bundle", "", "")
	var output, request *string
	if command != "create" {
		output = flags.String("output", "", "")
	}
	if command == "sign" || command == "sign-registration" {
		request = flags.String("request", "", "")
	}
	if flags.Parse(args[1:]) != nil || flags.NArg() != 0 || *wallet == "" || *bundle == "" || *pin == "" || !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stderr.Fd())) {
		return godtestnet.ErrWallet
	}
	state, err := term.GetState(int(os.Stdin.Fd()))
	if err != nil {
		return godtestnet.ErrWallet
	}
	defer term.Restore(int(os.Stdin.Fd()), state)
	// Restore terminal echo even if interrupted during a hidden read. This
	// command has no node service, writes in progress or submission to unwind.
	signals, finished := make(chan os.Signal, 1), make(chan struct{})
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	defer close(finished)
	go func() {
		select {
		case <-signals:
			_ = term.Restore(int(os.Stdin.Fd()), state)
			os.Exit(130)
		case <-finished:
		}
	}()
	if _, err := fmt.Fprintln(os.Stderr, "Synthetic assets only. Use a fresh password; never reuse a personal wallet. Password (hidden):"); err != nil {
		return godtestnet.ErrWallet
	}
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return godtestnet.ErrWallet
	}
	defer clear(password)
	if command == "create" {
		fmt.Fprintln(os.Stderr, "\nConfirm password (hidden). Keep an offline backup of the encrypted file; there is no seed phrase or password recovery:")
		confirmation, err := term.ReadPassword(int(os.Stdin.Fd()))
		defer clear(confirmation)
		if err != nil || !bytes.Equal(password, confirmation) {
			return godtestnet.ErrWallet
		}
		if godtestnet.WalletCreate(*wallet, *bundle, *pin, password) != nil {
			return godtestnet.ErrWallet
		}
		return json.NewEncoder(out).Encode(map[string]bool{"synthetic": true, "realAssets": false, "encryptedWalletCreated": true, "submitted": false})
	}
	if command == "address" {
		if godtestnet.WalletExportAddress(*wallet, *output, *bundle, *pin, password) != nil {
			return godtestnet.ErrWallet
		}
		return json.NewEncoder(out).Encode(map[string]bool{"synthetic": true, "realAssets": false, "privateAddressFileCreated": true, "submitted": false})
	}
	approve := func(review godtestnet.WalletReview) bool {
		fmt.Fprintln(os.Stderr, "\nReview the derived sender, network, exact smallest-unit amounts and maximum fee. Signing does not submit:")
		encoder := json.NewEncoder(os.Stderr)
		encoder.SetIndent("", "  ")
		if encoder.Encode(review) != nil {
			return false
		}
		fmt.Fprintln(os.Stderr, "Type SIGN and press Enter to approve (hidden); any other input cancels:")
		approval, err := term.ReadPassword(int(os.Stdin.Fd()))
		defer clear(approval)
		return err == nil && bytes.Equal(approval, []byte("SIGN"))
	}
	var report godtestnet.SignReport
	if command == "sign-registration" {
		report, err = godtestnet.WalletSignRegistration(*wallet, *request, *output, *bundle, *pin, password, approve)
	} else {
		report, err = godtestnet.WalletSign(*wallet, *request, *output, *bundle, *pin, password, approve)
	}
	if err != nil {
		return godtestnet.ErrWallet
	}
	return json.NewEncoder(out).Encode(report)
}
