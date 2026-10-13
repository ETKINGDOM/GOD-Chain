package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/ETKINGDOM/GOD-Chain/internal/goddeploy"
	"github.com/ETKINGDOM/GOD-Chain/internal/godrh"
	"github.com/ETKINGDOM/GOD-Chain/internal/godtestnet"
)

func runLaunchSupervisor(command string, args []string, out io.Writer) error {
	if out == nil || command != "check-launch-service" && command != "prepare-launch-supervisor" || len(args)%2 != 0 {
		return godtestnet.ErrLaunchSupervisor
	}
	allowed := map[string]bool{}
	for _, name := range []string{"home", "signer-guard", "input", "input-pin", "candidate", "candidate-pin", "binary", "binary-pin", "arch"} {
		allowed[name] = true
	}
	if command == "prepare-launch-supervisor" {
		for _, name := range []string{"uid", "gid", "report-seconds", "memory-bytes", "cpu-percent", "tasks", "open-files", "start-seconds", "stop-seconds", "output"} {
			allowed[name] = true
		}
	}
	values := map[string]string{}
	for i := 0; i < len(args); i += 2 {
		name := strings.TrimPrefix(args[i], "--")
		if args[i] != "--"+name || !allowed[name] || values[name] != "" || args[i+1] == "" {
			return godtestnet.ErrLaunchSupervisor
		}
		values[name] = args[i+1]
	}
	if len(values) != len(allowed) {
		return godtestnet.ErrLaunchSupervisor
	}
	ip, err := godrh.SourceMaterialPin(values["input-pin"])
	cp, e := godrh.SourceMaterialPin(values["candidate-pin"])
	if err != nil || e != nil {
		return godtestnet.ErrLaunchSupervisor
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	var report godtestnet.LaunchServiceCheckReport
	if command == "check-launch-service" {
		report, err = godtestnet.CheckPrivateLaunchService(ctx, godtestnet.LaunchServiceCheckOptions{
			Home: values["home"], SignerGuard: values["signer-guard"], BinaryPath: values["binary"],
			BinaryPin: values["binary-pin"], TargetArch: values["arch"], Inputs: godtestnet.LaunchRehearsalInputs{
				InputPath: values["input"], InputPin: ip, CandidatePath: values["candidate"], CandidatePin: cp}})
	} else {
		numbers := map[string]uint64{}
		for _, name := range []string{"report-seconds", "memory-bytes", "cpu-percent", "tasks", "open-files", "start-seconds", "stop-seconds"} {
			v, e := strconv.ParseUint(values[name], 10, 64)
			if e != nil || strconv.FormatUint(v, 10) != values[name] {
				return godtestnet.ErrLaunchSupervisor
			}
			numbers[name] = v
		}
		report, err = godtestnet.PrepareLaunchSupervisor(ctx, goddeploy.LaunchSupervisorOptions{
			BinaryPath: values["binary"], BinaryPin: values["binary-pin"], TargetArch: values["arch"],
			Home: values["home"], SignerGuard: values["signer-guard"], Input: values["input"], InputPin: values["input-pin"],
			Candidate: values["candidate"], CandidatePin: values["candidate-pin"], UserID: values["uid"], GroupID: values["gid"],
			ReportSeconds: numbers["report-seconds"], MemoryMaxBytes: numbers["memory-bytes"], CPUQuotaPercent: numbers["cpu-percent"],
			TasksMax: numbers["tasks"], OpenFiles: numbers["open-files"], StartSeconds: numbers["start-seconds"], StopSeconds: numbers["stop-seconds"]}, values["output"])
	}
	if err != nil || json.NewEncoder(out).Encode(report) != nil {
		return godtestnet.ErrLaunchSupervisor
	}
	return nil
}
