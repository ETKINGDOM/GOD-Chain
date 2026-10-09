# GOD Chain test explorer

The local test explorer browses committed stored blocks and summarizes actual native and EVM transactions on a reviewed synthetic network. Open `/explorer.html` through the companion's selected loopback listener. Supply the observer RPC and expected native chain ID from the reviewed bundle; no network or public deployment is preset.

The separately authorized public synthetic portal also serves a restricted block and transaction explorer. Its wallet links GOD and test NFT TX IDs to this explorer using same-origin fragments. NFT transactions appear as committed GOD execution summaries targeting the configured collection; the wallet's independent NFT checker verifies the collection event, token ID and current owner. An explorer execution summary alone is not proof of NFT ownership. Full NFT indexing and arbitrary collection support are not supplied.

Connect explicitly, load the latest ten blocks and select a block or enter an exact committed height. Each block page shows up to twenty transaction summaries. Inspect a known hash in the correct namespace, or query current committed GOD and G balances. No automatic polling, browser storage, RPC signing or external analytics is used. Disconnect clears operational values.

## Bounded explorer methods

Height, count and offset arguments are canonical decimal strings, except the `latest` height marker. They never pass through floating-point coercion. The existing restricted synthetic RPC retains its host, exact-origin, size, request and connection limits.

| Method | Parameters | Result and limit |
| --- | --- | --- |
| `god_blocks` | Starting height or `latest`, count | Descending stored block summaries, count 1 to 10, explicit next height |
| `god_block` | Height or `latest`, offset, limit | Metadata and paginated native/EVM summaries; limit 1 to 20, explicit next offset |
| `god_transactionDetails` | `ethereum` or `native`, known hash | One indexed committed summary, or null for an unknown/uncommitted result |

The Ethereum namespace uses the signed EVM hash. The native namespace uses the GodCometBFT wire hash and can also identify an EVM envelope's consensus record. They are not interchangeable encodings. Summaries preserve consensus indexes; EVM receipts retain their separate Ethereum-lane indexes.

Details verify stored block membership, transaction bytes and execution results before returning. Transactions ahead of committed application state are not reported as committed. Missing/pruned stored data can yield unavailable results; this is not an archive service. Results remain unverified RPC claims, not authenticated light-client proofs.

## Execution status and privacy

Native summaries expose supported operations, sender, recipient or validator, amounts, fees, SDK outcome and gas. EVM summaries expose sender, recipient, value, sequence, calldata byte count and actual execution outcome. `succeeded`, `failed` and `sdk-failed` are distinct. Inclusion and SDK success do not become fabricated EVM payment success.

The pinned SDK can report `gasWanted = "-1"` for a compatible SDK failure before
its gas budget was established. The adapter preserves this only when SDK code
is nonzero, and the participant display renders it as `unavailable`. Other
negative gas, success with this marker, inconsistent code/success flags and
relabeled execution are rejected. This marker is neither negative used gas nor
a refunded or charged fee; it does not create an execution receipt.

Explorer methods omit memos, calldata bytes, events, logs, raw execution errors and unknown message payloads. The browser renders fixed summary fields as text. This reduces accidental display but does not make public-chain data private: raw block and receipt interfaces can still expose submitted payloads. The application must encrypt private faith text before submission and protect its decryption keys.

An unknown hash remains pending, unsubmitted, pruned or unknown; it gets no confirmation claim. Verify actual execution and independently query the intended recipient balance. Block hashes are GodCometBFT identifiers, not fabricated Ethereum RLP headers or state roots.

## Public synthetic portal tools

The public portal has a checksum-checked watch-only account lookup. Both
returned address encodings must identify the requested account, metadata and
units must be bounded, and the account view must not precede the freshly queried
network height. Invalid, mismatched, stale or superseded results clear balances
and copy/share controls. This remains a service-consistency check, not a proof.

Exact `/#account=<canonical-native-address>` fragments open the same lookup on
reload. The connected wallet and local extension source link to this public view; the
extension uses only its reviewed testnet site origin. Public links reveal the
address, never unlock a wallet and contain no backup or passphrase. Save the
encrypted backup before switching tabs, which locks the wallet.

**Recent account activity** scans a manually selected starting height and up to
five consecutive stored blocks. It checks only the first twenty transaction
summaries in each block, using at most eight existing read-only RPC calls. The
display states the checked range, scanned count, matching count and omitted
transactions. **Scan five earlier blocks** checks the next bounded range only
when selected; there is no automatic history crawl.

Matching uses checksum-checked sender and recipient identities in supported
native operations and compatible GOD summaries. Rows distinguish incoming,
outgoing and self activity, preserve failed execution and link to the existing
transaction checker. Contract calls do not imply an NFT transfer or payment.
The scanner does not decode contract events, internal transfers, NFT ownership,
memos, calldata or private faith text. A missing match is not proof of no
activity. Inconsistent block membership, pagination, outcome or network data
rejects the whole range; changing the watched address clears earlier results
and suppresses late replies. This is a bounded service view, not a full address
index, archive or authenticated proof.

**Network check** makes at most three bounded read-only calls: network identity,
observer liveness, then network identity again. It runs only on request, does not
probe signing services and reports an explicitly dated service snapshot. A
failed or changed network discards earlier observations; an unavailable check
does not retain a responding result. Peer counts cannot establish operator
independence or geographic distribution.

**Download redacted report** produces a local JSON file with a fixed primitive
allowlist: check time, result categories, block heights, peer count and node-state
flags. It excludes hostnames, IPs, network/account/transaction/claim identifiers,
balances, signing material, private text and raw errors. Unknown provider fields
are not copied. Nothing is uploaded automatically. The English first-test guide
covers backup, claim/TX checks, reviewed transfer, restricted NFTs, recovery and
unknown-outcome handling. These tools are served by the public synthetic pilot;
their availability does not establish live adoption, sustained reliability or
independent validation. The extension's public activation remains separate.

## Opt-in durable address-history foundation

The included opt-in local CLI indexes selected complete stored blocks through the
existing observer reads. It validates every twenty-summary page, then persists
compact transaction identifiers, execution outcomes, supported address
membership and a checkpoint in one synchronous database transaction. It does
not store memos, calldata, raw wire, events, logs or private faith text. Queries
return at most twenty address matches with an exclusive, stored-anchor cursor.
Native and compatible encodings of the same account share one address index.

Private race checks cover 20/20/5 block and address pagination, failed-page and
failed-write rollback, abrupt process exit between blocks, corrupt data,
changed-source rejection and explicit retained-range coverage. Actual persistent
local consensus accepted a synthetic transfer; its matching indexed record
survived index reopen and observer restart. A bounded Linux acceptance indexed
ten already committed public-pilot blocks and recovered an existing native
exit record once across separate sync/query invocations. It made no claim or
submission and changed no existing service process, listener or gateway config.

That earlier acceptance covered the local CLI, not a public history API, complete
genesis backfill, NFT event index, authenticated proof or accepted production
database. Its production source and [usage guide](HISTORY.md) are included in
the reviewed core scope; private tests and operational data remain excluded.
It counts unsupported summaries explicitly and never turns an empty range into
proof of no activity. The bounded pilot deployment below is separate;
capacity/retention, power-loss acceptance and independent review remain open.

## Bounded public retained-history query and browser integration

An independent query-only loopback boundary and opt-in browser panel are now
deployed on the explicitly authorized synthetic pilot. Every response declares
the retained range, stored block anchor,
unsupported-summary count, index reconciliation time and fresh observer-check
time. Queries recheck network identity and current/older retained anchors; they
reject stale data, missing source blocks, malformed requests and file-lock
conflicts instead of returning fabricated empty pages. First/older pages are
manual, limited to twenty matches and never merged across stored anchors.

The exact version-one preview configuration still leaves this panel hidden.
The deployed version-two configuration additionally binds the reviewed bundle
digest and fixed same-origin
`/history/` route. The client validates addresses, schemas, ordered rows,
execution outcomes and cursors, omits credentials and browser storage, and
discards replies superseded by an address or network change. It renders only
the compact allowlisted fields. Unknown transaction and faucet IDs are not
modified. This is not a proof, NFT event index or historical balance view.

Private desktop/mobile fixtures and actual persistent local consensus queried
one separately signed synthetic transfer before and after observer recovery.
Those local checks did not activate a public route or participant asset. A
separate authorized deployment now supplies a bounded working-index sync timer,
verified atomic read-only snapshots and an exact-path restricted HTTPS proxy.
Actual hardened Linux query/writer concurrency, public request refusal and
read-rate limits passed. Desktop and mobile-width HTTPS browser checks recovered
an already committed TX through the new panel without signing or submitting.
The original chain processes and wallet/Chrome bytes were preserved. Coverage
remains partial and catching up; complete backfill, NFT events, sustained
capacity/load and storage-failure acceptance remain open. This query/build
addition still requires a separate public source-scope review before GitHub
upload. See [HISTORY.md](HISTORY.md).

The separate offline retained-snapshot audit now checks declared block/record
coverage and logical reverse-index consistency without source reads or writes.
Isolated restore copies preserved a real local-consensus TX and its checked
query result; a private pilot snapshot also passed a local read-only scan with
unchanged bytes. It is not a public explorer route, live restore, physical-page
check, authenticated backup or complete address/NFT history. The existing public
website and binaries are unchanged by this tool addition. See the
[storage audit guide](HISTORY.md#offline-retained-snapshot-storage-audit).

## Local NFT event projection foundation

The separately deployed collection-history page retains strict timestamp checks.
A query outside the five-second future/thirty-second past window clears activity
and pagination and shows an explicit English time-window warning. A network-time
refusal during initial configuration leaves history disabled and explains the
need to check automatic device time and reload. These messages do not adjust
either clock, relax freshness, automatically retry, sign or confirm an NFT event.
Observed device/server time disagreement remains an open acceptance issue.

A separate pure helper now checks bounded supplied receipts for the configured
synthetic collection and produces compact mint/transfer/self-transfer records.
Participants are taken from contract events rather than transaction signers;
approved-operator and nested transfers do not widen public signing permissions.
It binds receipt-lane indexes separately from consensus indexes, rejects
malformed or oversized inputs as a whole and retains no raw event payload.
Results explicitly decline full history and current-ownership proofs. A separate
collection-bound `nft-history.db` now supplies local atomic receipt/event/address/
token storage, immutable namespace/capacity policy, pinned twenty-event pages
and a read-only logical retained-range audit. Private reopen, between-commit
process interruption, isolated restore and committed SDK execution acceptance
passed. Stored pages deliberately decline fresh-source checks and current
ownership. The separate opt-in loopback acquisition now reads all consensus pages,
exact retained-height collection code and matching compatible receipts with
bounded schemas, pacing and pre/post anchor/network checks. Actual persistent
local consensus mint/send history survived observer/index recovery and isolated
restore. SDK failures now retain an explicit separately reconciled consensus
rejection, not an invented receipt or event. Missing or contradictory evidence
still stops the block. The private package fixtures also passed on Linux under the existing
low-privilege user with isolated loopback, no-new-privileges, memory write/execute
protection and bounded resources. Those package fixtures alone are not public
query or browser acceptance. A separately authorized public collection-history
page now uses its own read-only query service and atomically published images;
the retained address database and wallet/extension assets remain unchanged.
Token and address filters return at most twenty compact events with pinned
cursors, TX links and explicit partial coverage. Current network and retained
header/runtime consistency checks do not freshly re-fetch every event, authenticate
finality or prove current ownership. Snapshot/source drift yields unavailable,
not an empty-history claim. See the NFT history boundary (separate pilot guide, not included in this core snapshot)
and independent storage (separate pilot guide, not included in this core snapshot), plus
source acquisition (separate pilot guide, not included in this core snapshot) and
public collection history (separate pilot guide, not included in this core snapshot).

## Remaining explorer work

Full public account/NFT history, independent index-service operation, general contract verification, historical balances, complete historical validator indexing, proofs, subscriptions and sustained load testing remain separate work. The existing bounded registration directory is not historical signing evidence. The secured public synthetic pilot is a bounded alpha, not a production explorer or proof of independent validator operation. Broader cross-host and production acceptance still require the target environment described in [DEPLOYMENT.md](DEPLOYMENT.md).
