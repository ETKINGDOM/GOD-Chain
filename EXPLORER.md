# GOD Chain test explorer

The local test explorer browses committed stored blocks and summarizes actual native and EVM transactions on a reviewed synthetic network. Open `/explorer.html` through the companion's selected loopback listener. Supply the observer RPC and expected native chain ID from the reviewed bundle; no network or public deployment is preset.

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

## Remaining explorer work

Full account history, independent indexing, contract verification, historical balances, validator directory, proofs, subscriptions, sustained load testing and secured public hosting remain separate work. This is a first local test explorer, not a production explorer. Linux runtime and cross-host public-test acceptance require the target environment described in [DEPLOYMENT.md](DEPLOYMENT.md).
