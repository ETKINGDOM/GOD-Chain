# Keyless retained-history CLI

`godhistory` is an opt-in synthetic CLI for bounded indexing, status, twenty-row
address pages and offline logical audit. It opens no listener, signs nothing and
installs no background job. Separately deployed pilot history queries/browser
integration are not part of this exact source snapshot. No database, endpoint,
address, key or binary is distributed.

## Coverage and storage

Complete selected blocks are read from an explicitly configured numeric-loopback
observer through only `god_network` and `god_block`. Heights, indexes, parent/
time continuity, counts, hashes and supported address encodings are checked.
One complete block, compact records, address membership, deduplication keys and
progress commit synchronously together. Memos, calldata, wire, arbitrary messages,
events, logs and faith text are not retained. Native summaries cover six staking/G
scopes; compatible summaries do not prove internal transfers or successful payment.

Pages declare retained range, anchor, source-check time and unsupported-summary
count. Twenty rows plus one look-ahead use an exclusive stored-anchor cursor.
Partial coverage is not full history; empty does not prove inactivity. Reads
never clear, retry or replace an unknown submission.

One sync processes 1–128 blocks with at most 128 summaries per block in pages of
twenty, paced at no more than four source requests per second. Errors are not
retried automatically. Interrupted/rejected reconciliation preserves earlier
complete progress and disables queries without rewinding or repairing it.
Explicit ceilings are 1,000,000 blocks, 100,000 transactions and a 512-MiB file
guard, not measured capacity guarantees or permission to expand existing policy.

## Explicit private invocation

```sh
go mod download
make check
make build-history-index
```

Use unchanged pinned dependencies and the checksum-bound lifecycle build. Home
must already be owner-only and contain no unrelated files. Only the index
database can be created; unsafe/differently configured storage is refused, not
reset. Accepted Linux/macOS ownership paths are supported; other ACL paths remain
disabled. Operational values below are supplied privately, not public defaults.

```sh
./build/godhistory sync --home "$GOD_HISTORY_HOME" \
  --upstream "$GOD_LOOPBACK_OBSERVER_RPC" \
  --chain-id "$GOD_NATIVE_CHAIN_ID" \
  --compatible-chain-id "$GOD_COMPATIBLE_CHAIN_ID" \
  --bundle-sha256 "$GOD_REVIEWED_BUNDLE_SHA256" \
  --first-height "$GOD_HISTORY_FIRST_HEIGHT" \
  --max-blocks "$GOD_HISTORY_BLOCK_BUDGET" \
  --max-transactions "$GOD_HISTORY_TRANSACTION_BUDGET" \
  --limit "$GOD_HISTORY_BATCH_SIZE"
```

`status`, `page` and `audit` take the same namespace/coverage flags, without an
upstream, and cannot create storage. `page` also requires `--address`; later pages
need all four returned cursor fields: `--snapshot-height`, `--snapshot-hash`,
`--before-height`, `--before-index`. Page output intentionally contains public
identifiers, never signing material.

## Offline retained-snapshot storage audit

`audit` scans a trusted owner-only copy read-only, checking declared blocks,
transactions and both reverse indexes. Unknown buckets, missing/extra/orphan
rows and inconsistent counters/links fail closed. Work has a thirty-second
deadline and configured bounds; failures emit no successful partial report.
It makes no source read, repairs no data and never enables queries.

```sh
./build/godhistory audit --home "$GOD_HISTORY_BACKUP_COPY" \
  --chain-id "$GOD_NATIVE_CHAIN_ID" \
  --compatible-chain-id "$GOD_COMPATIBLE_CHAIN_ID" \
  --bundle-sha256 "$GOD_REVIEWED_BUNDLE_SHA256" \
  --first-height "$GOD_HISTORY_FIRST_HEIGHT" \
  --max-blocks "$GOD_HISTORY_BLOCK_BUDGET" \
  --max-transactions "$GOD_HISTORY_TRANSACTION_BUDGET"
```

Use a private copy of a closed database or an immutable published snapshot,
not an ordinary file copy of a live writer. Keep and verify a trusted byte digest
separately before opening a restored copy. Fresh source/anchor reconciliation
and any switch require separate steps; the CLI copies or activates nothing.
Reports expose aggregate counters/coverage, distinguish stored reconciliation
from fresh checks and retain false complete-history/public-route/real-asset flags.

Internal links cannot authenticate coherently rewritten data or reconstruct
participant semantics. Audit is not a physical-page scan, backup authentication,
power-loss test or finality proof. Private pagination, interruption/reopen,
storage-refusal and local consensus checks are summarized in
[VERIFICATION.md](VERIFICATION.md); private tests are excluded. Complete backfill,
NFT events, sustained traffic/capacity and physical recovery remain release gates.
