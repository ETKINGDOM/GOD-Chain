# Restricted native-operation candidate

`godnative` is an explicitly configured, keyless synthetic service for delegate,
undelegate, claim settled G, transfer G, contribute GOD to pending liquidity,
and voluntary G redemption. Included source covers admission, committed-state
checks, result lookup, bounded validator/unbonding reads, durable reservations
and offline maintenance. It contains no wallet or signing key. Other gateways
and browser/Chrome signers are separate pilot components, not included here.

## Build only

Use Go 1.25 or newer; verification used Go 1.26.8 and unchanged module pins.

```sh
go mod download
make check
make build-native-gateway
make build-native-gateway-linux
```

These commands do not initialize storage, start/install a service, replace the
pilot or activate assets. Cross-compilation is not runtime acceptance. Private
tests and their targets are excluded; see [VERIFICATION.md](VERIFICATION.md).

`cmd/godbuild -native-gateway` retains the checksum-bound GodCometBFT worker-
join overlay and verifies the unchanged `cosmossdk.io/log` v1.6.1 module. Two
pinned logging files use standard-library JSON through an exact ignored module
mirror and compiler overlay. Changed pins, extra/altered mirror files, symlinks
and Sonic/base64x gateway dependencies are refused. Original caches, pins and
consensus code remain unchanged. This avoids runtime-generated executable code;
it does not relax Linux write/execute-memory denial or certify a service.

## Explicit private configuration

Operational values have empty/zero defaults. The upstream is numeric-loopback
HTTP at its root path; the distinct listener is numeric loopback on a nonprivileged
port. One canonical HTTPS website origin and optionally one exact reviewed
Chrome origin are admitted. An origin is policy, not authentication. Gas is a
fixed reviewed 50,000–2,000,000 limit with positive fee floor and a 0.01 synthetic
GOD offered-fee ceiling. These are gateway restrictions, not production economics.

```sh
./build/godnative \
  --listen "$NATIVE_LISTEN" --upstream "$OBSERVER_RPC" \
  --origin "$TEST_WEBSITE_ORIGIN" \
  --chain "$SYNTHETIC_CHAIN_ID" --compatible-chain "$COMPATIBLE_CHAIN_ID" \
  --gas "$REVIEWED_NATIVE_GAS" --min-price "$REVIEWED_MIN_AGOD_PER_GAS" \
  --attempt-directory "$NATIVE_ATTEMPT_DIRECTORY" \
  --attempt-capacity "$REVIEWED_ATTEMPT_CAPACITY"
```

Serving requires an already initialized owner-only store. A trusted reverse
proxy must set the exact upstream Host and replace `X-God-Client-IP` with a
checked client address. Never expose the listener directly or trust a browser-
supplied client-IP header. Cookies, authorization, compression, query strings,
ambiguous JSON and arbitrary messages are refused. No populated configuration,
endpoint, chain identity or address is supplied by this source snapshot.

## Admission and results

`POST /state`, `/unbonding`, `/validator` and `/validators` provide bounded
committed preparation reads. Components and final network replies must agree
on the exact commit/policy. Fixed GOD supply and settled/pending GOD/G buckets
are checked without a ledger scan. These are operator consistency checks, not
authenticated state proofs. Bonded registration is not signature, uptime,
independent ownership or geography evidence. `--validator-only` cannot open the
attempt store or reach submission/status routes.

`POST /submit` accepts only an exact lowercase SHA-256 hash and signed wire,
at most 2 KiB decoded. The codec rebuilds one allowlisted message and the complete
direct-signature envelope byte-for-byte, checking low-S signature, chain, account
number, shared sequence, gas, fee and canonical roles. No memo, batch, fee grant,
arbitrary contract call, reserve release, issuance, validator creation or bridge
message is accepted. Freshness and state are rechecked before one upstream attempt.

| Result | Meaning |
| --- | --- |
| rejected / not-submitted | This request did not reach node admission; it does not resolve a prior unknown attempt |
| not-admitted | Only the exact correlated pre-ante commit-boundary reply; no automatic retry |
| submitted | Admission acknowledgment, not committed success |
| unknown / blocked-check-status | Keep the exact hash and check it; do not sign a replacement |
| confirmed / failed | Matching committed inclusion with success / nonzero execution code |

`POST /status` is read-only and matches network, exact hash, inclusion, approved
operation and result. Missing/malformed/mixed data stays unknown. Offered fee
does not prove the charged amount. Unbonding initiation or a time-reached entry
is not payout. Only successful GOD redemption burns G; failures retain it.

Work is bounded to three in-flight slots, explicit deadlines/budgets and at most
1,000 RAM-cached hashes. Status/submission have separate per-IP cooldowns under
the shared global budget. Reads never release guards.

## Durable inventory and offline tools

Initialization uses a separate `--initialize-attempts` invocation with the same
private policy flags. The directory must already be empty, absolute, canonical,
current-owner and mode 0700. The database is mode 0600, single-link, nonsymlink
and exclusively locked. Repeated `--blocked-hash` flags can seed at most 1,000
exact reviewed legacy uncertain hashes. Serving cannot initialize or repair it.
Only policy metadata and blocked hashes are stored: no wire, account, IP,
timestamp, provider reply, wallet or key. Linux/macOS ownership paths are
supported; other ACL paths fail closed.

The exact hash commits synchronously before the one upstream submission.
Reopen scans the bounded inventory without loading every hash into RAM. A crash
between reservation and send may block an unsent operation: cautious admission
protection, not an exactly-once queue. Only the exact current-request pre-ante
deferral can synchronously delete its own record. An uncertain write latches
refusal of fresh submissions; status/time cannot reset it or delete a record.
Chain account sequences remain the execution replay protection.

Capacity defaults to 1,000 with a four-MiB file guard. Explicit initialization/
offline copy permits up to 100,000 records; larger capacities use a 32-MiB guard.
Version-two metadata binds the exact serving capacity. Version-one stores retain
their original 1,000 capacity without rewrite. Full storage refuses admission;
there is no pruning or disk expiry.

`--audit-attempts` exclusively opens an existing store in a supervised child,
checks every logical record and the pinned database's page/freelist invariants,
then emits only aggregate schema/count/capacity/remaining/file-size fields,
`pagesChecked` and logical/file digests. The parent discards worker stderr,
rejects more than 4 KiB of stdout or noncanonical/partial reports and kills a
worker exceeding twenty seconds. Panic, failed exit and timeout return a static
refusal, never diagnostics or a partial certificate. Run offline tools as an
unprivileged OS-sandboxed account with hard memory/process limits, no network
access and no access to node/signing data. File bounds/timeouts are not a sandbox.
The internal worker flag is not an operator entry point; `CheckAttempts` must
only run in a supervised offline process, never a node/gateway. The package's
logical-only `AuditAttempts`/`CopyAttempts` APIs report `pagesChecked: false`.

`--verify-attempts` additionally requires `--expected-source-sha256` from an
independently retained reviewed checkpoint and accepts only the exact image.
A valid older image is refused against the latest approved checkpoint. Do not
compute that pin from the backup being verified or overwrite the current pin
with a backup's value. An unpinned audit cannot establish freshness. Checkpoint
authenticity/completeness/custody remain operator gates, not automatic restore
or rollback-proof storage.
`--copy-attempts-to` requires an existing empty owner-only destination, equal/
larger `--copy-attempt-capacity`, and the reviewed source file digest through
`--expected-source-sha256`. Supply the same private policy flags; these modes
are mutually exclusive with serving and initialization.

All source hashes and target metadata enter the first initialization transaction
atomically. Source before/after file audits must match; target count and logical
inventory digest must match. Logical digests exclude capacity/schema; physical
digests are separate. The CLI checks source pages before target creation and
target pages before reporting success. No mode starts a listener, signs, submits, repairs, shrinks,
overwrites or activates a copy. Failed copies may leave unaccepted private
diagnostic artifacts; retain the original and review separately.

## Deployment and durability limits

The deployed native pilot still uses its earlier volatile reservation map.
This upload does not replace it. Cutover needs a complete reviewed legacy
inventory, ingress drain, preservation of every unknown ID, matching private
policy/ownership/service paths and accepted recovery. A few known IDs cannot
prove that the old process's complete inventory was captured.

Private crash/atomic-copy fixtures and eighteen isolated non-root Linux offline
cases passed with memory/network/signing-storage confinement, including exact
restore, stale-image refusal and static rejection of both page faults. All
sixteen existing services/timers, service PIDs and listening sockets remained
unchanged. These checks are not public HTTP migration, power
loss during fsync, physical-storage certification, authenticated checkpoint custody,
sustained capacity or independent security review. The new pinned-image check
can reject a stale valid backup against an independently retained current pin;
it cannot detect rollback of both database and pin or recover omitted records.
Mainnet, RH bridging and
real assets stay disabled. See [ROADMAP.md](ROADMAP.md).
