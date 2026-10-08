# GOD Chain read-only history index foundation

This opt-in local development tool is not a public explorer release, archive,
NFT index, authenticated proof or production activation. It uses the existing
observer's `god_network` and `god_block` methods only. It opens no listener,
runs no background job, signs no transaction and reads no node or wallet key.
The reviewed public-core scope includes this production CLI and its index
module, not private tests, databases, operational settings or binaries.
Browser history integration and a public history route are not supplied.

## Stored data and explicit coverage

The tool reads every transaction page of each selected block, validates exact
height/index, hashes, parent continuity, time, counts, execution outcome and
supported sender/recipient encodings, then commits one complete block and its
address rows/checkpoint atomically. It stores compact public transaction
identifiers, namespace, inclusion position, execution category and address
membership. Memos, calldata, raw wire, arbitrary messages, events, logs and
private faith text are not projected into storage, reports or queries.

Supported native summaries cover the existing six staking/G operations.
Compatible summaries cover the directly signed sender and recipient, including
failed execution and contract calls. They do not infer successful payments,
internal transfers, NFT mint/send events, current ownership or account balances.
Unsupported native summaries are counted explicitly rather than decoded or
treated as a complete address record. The count is for the indexed range, not
the requested account.

Every result declares its configured first height, indexed-through anchor,
last successful source-check time and unsupported-summary count. A range that
starts after genesis is a partial retained range. An empty match is not proof
of no account activity. Historical index availability says nothing about a
wallet's unresolved submissions; no unknown ID is cleared, retried or replaced.

The reviewer-supplied native/compatible network identifiers and bundle digest
bind the private database namespace. Matching these settings and source replies
is not remote authentication of that bundle, genesis, headers or finality.

## Bounded reads and durable progress

- One explicit sync processes 1–128 new blocks, at most 128 summaries per block
  in pages of 20. Reads are paced to no more than four requests per second.
  Source errors are not automatically retried. Responses, deadlines, nesting
  and input schemas are bounded; ambiguous, redirected and real-asset replies
  are rejected.
- One address query uses a descending raw-address prefix seek: at most twenty
  matching rows and one look-ahead. Its exclusive cursor pins a stored block
  anchor. Append-only growth does not combine different query snapshots.
- Complete blocks, compact records, address membership, deduplication keys and
  checkpoint metadata share one synchronously committed database transaction.
  Failed pages and writes preserve only earlier complete progress.
- Each sync first clears the verification flag, checks the previous checkpoint,
  indexes its bounded range and verifies the final source/anchor before enabling
  queries. Interruption, changed source, regression or missing/pruned data leave
  queries unavailable. They never erase, repair or rewind the checkpoint.
- Explicit policy caps permit at most 100,000 blocks and 100,000 transactions.
  A conservative 512 MiB refusal/headroom check is not an OS filesystem quota
  or a measured capacity promise. Dense blocks can be rejected before their
  append; review capacity rather than silently pruning history.

The existing pinned bbolt dependency is reused without changing the module
lock. This does not establish its suitability for production, power-loss or
torn-write recovery. Tests of graceful reopen and abrupt process exit between
blocks do not replace storage/adversarial testing and sustained measurements.

## Private invocation

Use the reviewed Go 1.26.8 toolchain and checksum-bound lifecycle build. The
directory must already exist, be owner-only, and contain no unrelated file.
Only `history.db` may be created; an existing empty, shared, linked, malformed
or differently configured database is rejected rather than reinitialized.
File handling is enabled only on accepted Linux/macOS permission paths; other
runtime ownership/ACL behavior remains disabled.

```sh
make build-history-index
```

All operational values below are supplied privately, not public defaults:

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

`status` opens an existing database read-only and emits fixed progress fields
without network, account, transaction or bundle identifiers. `page` additionally
requires a public `--address`; a next page needs all four returned cursor fields
through `--snapshot-height`, `--snapshot-hash`, `--before-height`, `--before-index`.
Page output intentionally contains public transaction identifiers, never keys
or private payloads. Both commands require the same reviewed namespace/coverage
flags as sync, but cannot accept an upstream or create a database.

No command replaces an existing directory, launches a daemon, purchases storage,
changes consensus/economics, activates mainnet or establishes RH backing.

## Recorded private acceptance

Developer-run race checks cover complete 20/20/5 transaction and address pages,
anchored queries during append, native/compatible identity matching, synchronous
rollback, abrupt process exit between blocks and source/database rejection.
Persistent local four-validator consensus accepted one synthetic transfer; its
sender/recipient history survived index reopen and observer restart. The index
itself never signed or submitted that fixture transaction.

The complete local Go regressions and unchanged 101-check wallet/browser suite
also passed. These checks used allowed localhost/disposable browser fixtures;
initial sandbox permission failures were not counted as passes. The wallet
checks are not new public-chain claims/signatures, physical-device acceptance,
settled-G payout or a complete public-testnet security review.

An isolated Linux amd64 candidate indexed ten already committed pilot blocks
over two bounded sync invocations. A previously committed native exit record
appeared exactly once before and after reopen. The private database used
65,536 bytes and the measured child-process peak RSS was 65,280 KiB for this
small acceptance run. These are not capacity, sustained-load or ordinary-PC
requirements. All twelve existing service processes, TCP listeners and the
nginx configuration remained unchanged. No faucet claim, transaction submission,
public route, complete backfill or NFT-history acceptance occurred.

Ten additional Linux CLI cases rejected unsupported startup/reset/asset
operations, wrong network/bundle binding, an invalid address and an upstream
argument on read-only status. Each returned the fixed generic error and no
stdout. Database bytes, all twelve existing service processes and nginx
configuration remained unchanged. These are local CLI rejections, not public
HTTP admission or physical-storage failure acceptance.

## Remaining delivery gates

Full retained-range backfill and measured catch-up; independently constrained
public query service and exact-origin abuse controls; browser coverage/cursor
display; NFT event indexing and historical ownership distinctions; retention
and disk-budget acceptance; corruption, power-loss and restore drills;
adversarial review; and sustained intended-host measurements. Mainnet and real
assets remain disabled until their separate release gates are approved.
