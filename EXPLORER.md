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

## Remaining explorer work

Full account/NFT history, independent indexing, general contract verification, historical balances, validator directory, proofs, subscriptions and sustained load testing remain separate work. The secured public synthetic pilot is a bounded alpha, not a production explorer or proof of independent validator operation. Broader cross-host and production acceptance still require the target environment described in [DEPLOYMENT.md](DEPLOYMENT.md).
