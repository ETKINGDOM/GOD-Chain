# GOD Chain core verification

Developer-run verification on 2026-10-03 used macOS arm64 and Go 1.26.8. It covers synthetic local execution, staking, consensus and isolated bridge accounting. It is not an independent audit, deployed network, source-finality proof or mainnet safety certification. Private tests and their generated inputs are not distributed in this source snapshot.

## Observed local checks

- All 108 substantive named tests passed with the optional compiled-custody checks enabled. Race detection, module checksum verification, Go vet and formatting checks passed.
- Native tests exercised six G operations, ownership, signatures, fee tampering, chain/account binding, replay, gas exhaustion, batch rollback and committed-state reload.
- Execution tests covered the three enabled Ethereum transaction families, shared native/Ethereum sequences, contract code and storage, revert fees, module-account protection, transfer ordering and self-destruction quarantine. GOD supply stayed fixed without bank mint/burn permission.
- Staking tests covered signed creation, delegation, unbonding and delayed validator updates. Reward checks covered operator commission, delegation shares, integer rounding, invalid commits, daily ceilings and absence of halt catch-up.
- Four separate loopback validator processes produced matching committed application state. Stored commits were independently checked against validator keys and powers. A signed contract deployment propagated over P2P and produced matching code and storage. After one validator stopped, three advanced; the absent validator's pending G did not increase.
- Isolated bridge checks used real God SDK stores and compiled custody bytecode with synthetic assets. They covered one-for-one conservation, approvals, replay, queue/delay/rolling limits, mutually exclusive paid/cancelled outcomes, transfer failures and hostile synthetic-token behavior. No RH connection or backing was verified.
- Separate 10-second fuzz windows exercised 16,089 bounded execution-envelope inputs and 13,597 strict node-genesis inputs without failure. These are parser inputs, not blockchain transactions or accounts.
- Combined-node private test binaries compiled with CGO disabled for Linux amd64/arm64, macOS amd64/arm64 and Windows amd64. Only macOS arm64 execution was checked. Cross-compilation is not foreign-platform runtime support.

These processes ran on one computer, not independent geographic operators. Direct application fixtures contain synthetic vote flags; only the separate consensus test provides actual signature evidence. Consensus databases were in memory. Durable signing, combined-node process restart and operating-system crash recovery were not verified in this milestone.

## Public source checks

The reviewed snapshot contains production-source candidates, schemas, generated messages, pinned module versions, checksums and required attribution. It retains the public whitepaper. Private tests, fixture contracts, operational scripts, genesis, addresses, keys, dependency folders, logs and binaries are excluded.

`make check` verifies the public Go source with vet, module checksums and compilation. It does not rerun the private test count, fuzz windows or custody-bytecode tests. Before upload, exact file scope, both commit identities, file bytes, commit messages and all reachable public history must pass publication screening. Screening does not establish the absence of every possible secret or replace independent review.

## Unverified release gates

Public startup and financial commands remain disabled. Durable signing and recovery, adversarial networks, misconduct evidence, nonburning penalties, offline suspension, approved emissions, governance, authenticated bridge messages, finalized-source observation and backed genesis remain incomplete. Ethereum RPC and persistent wallets are absent. No ordinary-computer minimum configuration or worldwide load capacity has been measured.
