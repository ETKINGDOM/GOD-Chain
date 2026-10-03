# GOD Chain project guardrails

- GitHub uploads must authenticate as `ETKINGDOM` and target `ETKINGDOM/GOD-Chain`.
- Git author and committer must both be `ETKINGDOM <337168971+ETKINGDOM@users.noreply.github.com>`.
- Before every push, verify the account, both commit identities, all uploaded files and all reachable history. Stop on failure; never fall back to another account.
- This repository contains planning documents and an early core source prototype. It is not an operational EVM node, mainnet, live bridge, staking service or independently audited protocol.
- The chain is `GOD Chain`; project-owned components are `God EVM`, `God SDK` and `GodCometBFT`. Default documentation language is English. Preserve required upstream imports, licenses, copyright and attribution without claiming independent invention.
- GOD has a fixed economic supply of 1,000,000,000. The reward keeper cannot mint or burn GOD. Equal bank supply does not establish RH backing.
- G has no fixed lifetime cap at this stage; reward issuance must follow bounded, auditable protocol rules, not user-supplied allocation messages. The 10,000 G daily ceiling remains a proposed prototype setting.
- Redemption is voluntary. Burn offered G only on successful GOD payment. Empty liquidity preserves G and its separately enabled uses. Preserve the selected 100 percent GOD gas-fee routing into pending reward funds unless the owner explicitly authorizes a substantive change.
- Keep G and GOD business writes in the same cached SDK transaction. Rejected ante validation does not commit fees or sequence; accepted transactions retain fees and sequence on business failure while rolling back business state.
- Native transaction accounts use canonical lowercase `god1` Bech32 and the same raw 20-byte identifier as the compatible EVM address. Validate ownership, checksums and roles; text conversion creates no wallet and moves no assets.
- Native signature prototypes do not enable Ethereum-format transactions, public RPC or a running validator network. Keep startup, real assets and unsupported production paths disabled until their integration and release gates pass.
- Publish only the reviewed production-source snapshot and approved documents with separate public history. Preserve the whitepaper. Do not import unrelated application source, private history, private test suites, fixtures, credentials, keys, populated wallet or contract addresses, development endpoints, runtime data, dependency folders, logs or binaries.
- Identify the project account only as ETKINGDOM in user-facing communications. Do not publish account-transition explanations.
- Private faith text and encryption keys must never enter public chat, telemetry or logs. Website deployment, chain activation, bridge custody, real-money operations and substantive economics changes require explicit authorization.

## CodeGraph

Use CodeGraph first if an existing `.codegraph/` index is present. Do not create an index without the user's request.
