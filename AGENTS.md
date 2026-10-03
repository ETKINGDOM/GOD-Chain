# GOD Chain project guardrails

- GitHub uploads must authenticate as `ETKINGDOM` and target `ETKINGDOM/GOD-Chain`.
- Git author and committer must both be `ETKINGDOM <337168971+ETKINGDOM@users.noreply.github.com>`.
- Before every push, verify the authenticated account, both commit identities, all uploaded files and all reachable history. Stop if verification fails. Never fall back to another account.
- This repository currently contains planning documents only. Do not present proposed mainnet, bridge, staking, rewards or hardware targets as implemented or validated.
- The chain name is `GOD Chain`. Use `God EVM`, `God SDK` and `GodCometBFT` as the execution, application and consensus component names. Default documentation language is English.
- Keep required third-party licenses, copyright notices and attribution. Preserve necessary dependency and interoperability identifiers without claiming independent invention of third-party technology.
- Public files and history must contain no credentials, keys, test addresses, populated wallet or contract addresses, local operational endpoints, private development material or account-transition explanations.
- Identify the project account only as ETKINGDOM in user-facing communications.
- Preserve whitepapers and unrelated local work. Private faith text and encryption keys must never enter public chat, telemetry or logs.
- Publish only reviewed, sanitized snapshots with separate public history. Do not import private application history or unrelated application source.
- Website deployment, chain activation, real-money operations, bridge custody and substantive economic changes require explicit authorization.

## CodeGraph

If a `.codegraph/` directory exists, use CodeGraph before textual code discovery. Do not create an index without the user's request.
