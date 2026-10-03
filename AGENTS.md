# GOD Chain project guardrails

- GitHub uploads must authenticate as `ETKINGDOM` and target `ETKINGDOM/GOD-Chain`.
- Git author and committer must both be `ETKINGDOM <337168971+ETKINGDOM@users.noreply.github.com>`.
- Before every push, verify the account, both commit identities, all uploaded files and all reachable history. Stop on failure; never fall back to another account.
- This repository contains planning documents and isolated execution, staking, consensus and bridge source prototypes. Local integration does not establish public operation, RH backing, mainnet readiness or independent audit.
- The chain is `GOD Chain`; project-owned components are `God EVM`, `God SDK` and `GodCometBFT`. Default documentation language is English. Preserve required upstream imports, licenses, copyright and attribution without claiming independent invention.
- GOD has a fixed economic supply of 1,000,000,000. The reward keeper cannot mint or burn GOD. Equal bank supply does not establish RH backing.
- G has no fixed lifetime cap at this stage; reward issuance must follow bounded, auditable protocol rules, not user-supplied allocation messages. The 10,000 G daily ceiling remains a proposed prototype setting.
- Redemption is voluntary. Burn offered G only on successful GOD payment. Empty liquidity preserves G and its separately enabled uses. Preserve the selected 100 percent GOD gas-fee routing into pending reward funds unless the owner explicitly authorizes a substantive change.
- Keep G and GOD business writes in the same cached SDK transaction. Rejected ante validation does not commit fees or sequence; accepted transactions retain fees and sequence on business failure while rolling back business state.
- Native transaction accounts use canonical lowercase `god1` Bech32 and the same raw 20-byte identifier as the compatible EVM address. Validate ownership, checksums and roles; text conversion creates no wallet and moves no assets.
- The isolated node accepts bounded Ethereum transactions and restricted native messages. It exposes no public RPC; synthetic loopback consensus is not a mainnet. Keep startup, real assets and unsupported production paths disabled until their release gates pass. Penalties, disk recovery and the observed shutdown repair are locally tested, not operator-grade certification. Node builds must use the checksum-bound lifecycle helper; never patch the original cache, change pins or bypass worker joins without review.
- Explicit synthetic BridgeGenesis mounts and initializes the ledger atomically from full reserve before staking. Zero BridgeApprovalGas leaves participant bridge routes disabled; only a separately approved positive bounded setting enables six authenticated node routes with binding version 4 and exact gas-policy authorization. Require complete funding reconciliation, separate node/quorum approvals, owner consent and consensus-key possession. Quorum attestations and read-only proposal digests do not independently verify source finality. Never relabel synthetic balances as backed genesis, allocate deposits twice, manufacture initial fees/stake or bypass authentication. Incompatible binding versions require reviewed migration, never reset or replacement signing keys.
- Defer transaction checking without ante processing or writes between finalization and commitment. Never wait under a mempool lock needed by commit, retry authentication failures as success or weaken ledger time guards to handle admission deferral.
- Publish only the reviewed production-source snapshot and approved documents with separate public history. Preserve the whitepaper. Do not import unrelated application source, private history, private test suites, fixtures, credentials, keys, populated wallet or contract addresses, development endpoints, runtime data, dependency folders, logs or binaries.
- Identify the project account only as ETKINGDOM in user-facing communications. Do not publish account-transition explanations.
- Private faith text and encryption keys must never enter public chat, telemetry or logs. Website deployment, chain activation, bridge custody, real-money operations and substantive economics changes require explicit authorization.

## CodeGraph

Use CodeGraph first if an existing `.codegraph/` index is present. Do not create an index without the user's request.
