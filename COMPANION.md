# GOD Chain Test Companion

The local test companion provides a participant entry point for the synthetic GOD Chain network. It displays committed network, account, delegation, block and transaction results, prepares unsigned funding, GOD-transfer and native-operation files, and exports feedback locally. The workbench pairs with a separate [encrypted terminal test wallet](WALLET.md) and a [bounded test explorer](EXPLORER.md). The browser never signs, broadcasts, operates an automatic faucet or starts a chain.

Use disposable accounts and assets with no monetary value. No RH bridge, mainnet balance conversion, guaranteed reward or real payment is enabled. The stack is God EVM + God SDK + GodCometBFT. Network operators should first follow [TESTNET.md](TESTNET.md).

## Start the local interface

Build with the reviewed toolchain and checksum-bound lifecycle build:

```sh
make check build
./build/godd companion --listen "$GOD_COMPANION_LISTEN"
```

Supply an explicit numeric loopback IP and an unused port above 1023. There is no default listener, public-hosting flag, RPC proxy, directory listing or access to node files. Start the chain separately. Open the selected local interface in a modern browser with JavaScript, BigInt and Fetch support.

The page contains no populated network, wallet address, contract setting or contact destination. Provide the observer RPC URL through a reviewed private operational channel. Use HTTPS for an eventual remote observer, or loopback HTTP for local testing. The observer must permit the interface's exact origin; do not disable host or CORS checks. Public hosting and gateway operation require separate review and authorization.

RPC requests occur only after participant actions. The page does not retain operational values in browser storage, collect wallet keys or send analytics. Disconnect clears interface values but does not revoke permissions in an external wallet. Closing the page is not a validator shutdown.

## Participant workflow

1. Verify the expected native and EVM chain identifiers and complete bundle digest with the operator. Enter the reviewed RPC URL, connect and inspect the synthetic flags, fixed supply, committed height and liveness. A responding endpoint is not a proof of correct state or quorum.
2. Create a fresh disposable encrypted wallet using [WALLET.md](WALLET.md) and paste its public address into the workbench. The optional injected-wallet button requests only its account address; it never signs, switches or adds networks. Native `god1` and EVM encodings identify the same account and balance, not a bridge.
3. Prepare a manual test-funding request using the reviewed bundle digest, recipient EVM address and an amount of up to five synthetic GOD. Downloading the file does not submit or approve it. Share it only through the operator's designated private channel.
4. After manual funding, query the actual Ethereum receipt and recipient balance. An admission response, missing result or signed file is not payment confirmation.
5. Query the account immediately before preparing a native operation. Select delegation, undelegation, settled-G claim, G transfer, GOD liquidity contribution or voluntary G redemption. Use an operator-reviewed validator address and fees. Download the unsigned request, then review and sign offline with the disposable test-wallet helper.
6. Submit the signed wire separately through the restricted node RPC. Inspect its actual inclusion result, then re-query the balance or delegation. A successful outer SDK result alone is not an EVM-success signal.
7. Record reproducible issues in a locally exported feedback file. Review it before sharing privately. The interface has no feedback-upload service or default recipient.

Downloaded operational files contain addresses and signing metadata even when they contain no key. Move them into an owner-only private directory and apply owner-only file permissions before using the command tools. Do not commit them or attach populated files to public issues.

## Manual test funding

An operator reviews the exact recipient, amount, chain identifiers and full bundle digest. Requests are not authenticated ownership claims or proof of distinct people. Keep a private approval record and independently inspect prior receipts before funding a repeated request. The per-request cap is not a rate limit or Sybil defense.

The offline `sign-funding` helper signs a plain legacy Ethereum transfer from an initial validator's separately held disposable account. That account must appear as an owner in the reviewed synthetic bundle; the consensus and P2P signing keys are never used. Funding transfers only existing unbonded test balance. It cannot mint, release reserve or create RH backing.

Query a fresh account sequence through the reviewed observer, verify the chain and choose the gas price. Native and EVM operations consume the same sequence. A stale sequence can invalidate the funding transaction.

```sh
./build/godd testnet sign-funding \
  --request "$GOD_PRIVATE_FUNDING_REQUEST" \
  --key "$GOD_OFFLINE_TEST_WALLET_KEY" \
  --output "$GOD_NEW_SIGNED_FUNDING" \
  --bundle "$GOD_BUNDLE_FILE" \
  --expected-bundle "$GOD_REVIEWED_BUNDLE_SHA256" \
  --nonce "$GOD_FRESH_FUNDING_SEQUENCE" \
  --gas-price "$GOD_REVIEWED_GAS_PRICE"
```

The helper bounds the transfer to five synthetic GOD, uses 21,000 gas, caps the offered transaction fee at 0.001 synthetic GOD, checks the configured minimum gas price, rejects self-funding and creates a new private output without replacement. These are test-tool limits, not approved mainnet economics or guaranteed execution. A recipient contract may fail with the plain transfer gas limit. Use disposable participant accounts and inspect the result.

The output contains the Ethereum hash and `rawTransaction`; its redacted report states `submitted: false`. Send the raw transaction separately through `eth_sendRawTransaction`, then check `eth_getTransactionReceipt` and the recipient's balance. On the explicit commit-pending rejection, retry the same signed wire after commitment. A timeout or disconnect leaves an unknown outcome; check the known hash before deciding whether to resubmit. Never produce a replacement nonce blindly or call a signed file a completed transfer.

The helper is not an encrypted wallet or custody service. Never export an existing personal wallet's key to use it. Keep the disposable account key offline and outside node, observer and web-server workspaces. Automated faucet keys, queues, authentication, abuse controls and operational reconciliation are separate work.

## Native operation files

The interface uses exact decimal strings for account number, sequence, amounts, gas, fee and deadlines. Human amounts allow at most eighteen fractional digits and never round silently. Delegation, undelegation and redemption require at least one whole unit of their respective asset in this interface. Chain and signing validation remain authoritative.

The request builder uses a fresh account query and rejects a changed identity or sequence. This does not reserve the sequence or authenticate RPC results. Review the request and pinned bundle, then use encrypted `wallet sign`. The earlier raw disposable-key `testnet sign` helper remains available for private operator fixtures; it is not the encrypted wallet. Browser-native signing and extension flows remain incomplete.

Undelegation uses the existing 21-day chain delay. G earned from signing contribution remains pending until daily settlement. Claiming pending G is not supported. Empty liquidity must preserve G; only a successful voluntary redemption burns the offered G. Redemption requests require positive minimum output and an explicit future deadline with a UTC offset. No exchange return is guaranteed.

## Viewer and issue handling

The original viewer reads stored blocks, EVM receipts and native results. The separate `/explorer.html` adds descending block pages, paginated native/EVM summaries and indexed known-hash details, with explicit failure/unknown states and no raw memo/calldata text. Full address history, historical balances, validator directory, proofs and replacement detection remain incomplete. See [EXPLORER.md](EXPLORER.md).

Feedback exports contain only a category and participant-entered reproduction steps. There is no automatic collection of endpoints, addresses, logs or wallet state. Basic text guards reject obvious secrets and operational values, but they cannot identify every sensitive statement. Do not include private faith text, credentials, mnemonic words, raw logs or populated configuration. Nothing is uploaded by the companion.

## Verification and remaining launch work

Private client guards and race-enabled node tests are excluded from this public source snapshot. Their observed checks and limits are recorded in [VERIFICATION.md](VERIFICATION.md); public build commands do not rerun them.

Two local browser rounds checked manual funding and native delegation against four independently initialized validators and an observer on one computer. Funding and delegation were signed and submitted outside the companion, then verified through actual inclusion results and committed queries. Desktop/mobile layout and an unknown transaction result were also checked. The wallet connection used an address-only mock, not a real extension. Detailed evidence is recorded in [VERIFICATION.md](VERIFICATION.md).

Local browser checks and synthetic node tests do not establish public-service reliability. Before public testing, complete target-host and cross-host acceptance, secured observer/gateway configuration, sustained UTC-day settlement, resource measurements, funding review and incident contacts. Graphical signing, extensions, recovery, full explorer indexing, an automated abuse-controlled faucet and installers remain separate deliverables. Test assets must never be presented as real GOD or converted to mainnet balances automatically.
