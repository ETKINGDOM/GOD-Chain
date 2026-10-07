# GOD Chain encrypted test wallet

The synthetic test wallet combines an offline terminal signer with the local browser workbench. It creates a new encrypted account, exports public native and EVM addresses, and signs GOD transfers or native staking and G operations after explicit terminal review. No account key enters a browser, RPC service or temporary plaintext key file. It is not a production wallet, browser extension, custody service or automatic broadcaster.

Use disposable accounts and assets with no monetary value. This tool cannot import a personal key or seed phrase, sign arbitrary contract calls, operate the RH bridge or convert test balances into mainnet balances. Start a reviewed synthetic network using [TESTNET.md](TESTNET.md), and serve the workbench using [COMPANION.md](COMPANION.md).

## Private wallet setup

Use the checksum-bound local build and a macOS or Linux terminal. Windows private-file operations remain disabled pending owner-only ACL acceptance. Set the variables below yourself to absolute private paths and the independently reviewed complete bundle digest. Keep the wallet directory outside node and web-server workspaces; its permissions must be 0700. New files are 0600 and existing files are never replaced.

```sh
./build/godd wallet create --wallet "$TEST_WALLET_FILE" \
  --bundle "$TEST_BUNDLE_FILE" --expected-bundle "$TEST_BUNDLE_SHA256"

./build/godd wallet address --wallet "$TEST_WALLET_FILE" \
  --bundle "$TEST_BUNDLE_FILE" --expected-bundle "$TEST_BUNDLE_SHA256" \
  --output "$TEST_ADDRESS_FILE"
```

The terminal asks for a unique password twice during creation. Passwords are hidden and accept 12 to 256 UTF-8 bytes, without leading/trailing whitespace, newline or NUL. Arguments, password files, environment values and piped passwords are not accepted. Use a strong unique passphrase rather than relying on the length check.

Back up the encrypted file and password offline. There is no seed phrase, password reset, password rotation, raw key export or automated recovery tool. Losing either backup means losing the account. The encrypted payload authenticates the private scalar and full bundle digest; a wallet cannot be rebound to another bundle. Create a fresh account for a different synthetic network.

The address command creates a new private address file. Its canonical lowercase `god1` and EVM encodings identify the same twenty-byte account and shared ledger. Copy only a public address into the workbench, request manually approved funding and verify the actual receipt and balance before testing.

## Review and sign

Query a fresh account sequence before preparing a request. GOD transfers and native operations share the same sequence; querying does not reserve it or authenticate a remote service. Avoid concurrent operations against the same sequence.

Download an unsigned request from the workbench, move it into the private directory and apply 0600 permissions. Independently verify the chain, bundle, sender, recipient or validator, amounts and fees. Then sign:

```sh
./build/godd wallet sign --wallet "$TEST_WALLET_FILE" \
  --bundle "$TEST_BUNDLE_FILE" --expected-bundle "$TEST_BUNDLE_SHA256" \
  --request "$TEST_REQUEST_FILE" --output "$TEST_SIGNED_FILE"
```

Unlocking is not approval. The terminal displays the derived sender and all relevant signed intent fields: chain, bundle, account number, sequence, gas, maximum fee and any redemption minimum/deadline. Amounts are exact smallest-unit integers; one GOD or G equals 1000000000000000000 units. Type `SIGN` only when the intent matches. Any other answer cancels without creating signed output. Stdout remains redacted; terminal review intentionally contains operational identities, so do not record or publish it.

The six native operations are delegate, undelegate, claim settled G, transfer G, contribute GOD to liquidity and voluntary G redemption. Signing reuses the restricted God SDK direct-signature builder. Expired redemption requests are rejected before approval; execution still checks the deadline and minimum output. The wallet caps offered fees at 0.01 synthetic GOD.

GOD transfers use a chain-bound legacy EVM signature, 21,000 gas, no calldata, a positive amount and an explicitly reviewed gas price. Native and EVM recipient checksums are validated; self-transfers are rejected. Contract recipients can fail with this plain-transfer gas limit. Arbitrary contract signing is outside this milestone.

## Submission and confirmation

The wallet never contacts RPC. A signed file is private operational material that anyone receiving it can submit. Share it only through your approved submission workflow. Signing and admission are not inclusion.

| Signed operation | Submission method | Inclusion check |
| --- | --- | --- |
| GOD transfer with `rawTransaction` | `eth_sendRawTransaction` | Actual EVM receipt and explorer Ethereum-hash details |
| Native operation with `wire` | `god_submitTransaction` | Native SDK result and explorer consensus-hash details |

Submit only to the reviewed observer RPC, inspect the [test explorer](EXPLORER.md) and re-query the intended recipient balance or delegation. SDK success does not imply EVM success. A missing indexed result is unknown, not success or proof of rejection. After a timeout/disconnect, check the known hash before resubmitting; only an explicit commit-pending non-admission result allows a simple post-commit retry. This milestone retains manual submission rather than accepting a key or blindly retrying in the browser.

## Storage and acceptance limits

Format version one fixes scrypt at N=262144, r=8, p=1 and a 32-byte derived key, with random 32-byte salt and a 12-byte nonce. Standard-library [AES-GCM](https://pkg.go.dev/crypto/cipher#NewGCM) authenticates the payload with a 256-bit key; the KDF uses the existing pinned [scrypt library](https://pkg.go.dev/golang.org/x/crypto/scrypt#Key). Untrusted files cannot select weaker or unbounded parameters. Strict bounded JSON, owner-only permissions and exclusive creation remain enforced.

Encryption does not protect a compromised device, malware, weak passwords or terminal capture. Buffer clearing is best-effort, not guaranteed secure erasure. Recovery/backup acceptance, password rotation, graphical signing, HD accounts, hardware wallets, extensions, external security review and sustained participant testing remain separate milestones. These tools do not authorize public deployment or real assets.
