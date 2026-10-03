# GOD Chain local node runtime

`internal/godnode` connects God EVM execution, God SDK accounts and staking, and GodCometBFT consensus in an isolated synthetic-asset runtime. Four separate validator processes have produced matching committed state on the same computer. This is local integration evidence, not a live GOD Chain, independent operators, RH backing or mainnet readiness.

## Execution and fixed supply

The runtime uses the locked execution keeper and Ethereum signature checks. It accepts legacy, access-list and dynamic-fee Ethereum transactions in a bounded SDK envelope. Contract deployment, bytecode execution and persistent contract storage use the same committed application state as GOD balances. Native participant transactions retain the reviewed single-owner direct-signature rules. Both paths share account sequences, so mixing transaction formats does not create a second nonce balance.

Native ecosystem precompiles, execution hooks, IBC, automatic ERC20 conversion, blob transactions and delegated-code transactions are not enabled. There is no Ethereum JSON-RPC server, persistent wallet, gas estimator or wallet interface. These omissions are explicit limits, not evidence of complete wallet compatibility.

The reference balance setter ordinarily changes bank supply while reconciling EVM account deltas. The GOD adapter instead stages those deltas and atomically transfers existing GOD from debited accounts to credited accounts. Debits precede credits, irrespective of address order. It neither borrows the bridge reserve nor grants mint or burn permission. Failed business execution rolls back the message cache. EVM gas refunds are paid before residual fees enter pending rewards; native signed fees retain their separate full-requested-fee rule.

EVM self-destruction-to-self can discard native value under Ethereum semantics. GOD Chain preserves that value in a protected quarantine account instead of burning it. This is an intentional native-value policy difference, not general Ethereum equivalence. Quarantine has no authenticated release or reward route. GOD supply remains 1,000,000,000 throughout execution. Fixed supply alone does not establish backing.

## Validators and staking

The runtime uses the locked staking keeper, with `agod` as the bond denomination, a maximum active set of 32 and a 21-day unbonding period. Signed creation requires at least 1,000 GOD of self-delegation and a fixed 10 percent commission. Signed delegation requires at least 1 GOD. Undelegation must not leave a positive position below 1 GOD. Consensus keys are Ed25519; account signing keys remain EVM compatible. Their roles are separate.

Only creation, delegation and undelegation are added to the native participant allowlist. Redelegation, commission edits, parameter updates, governance and generic bank messages remain unavailable. A local resource bound allows at most 256 delegators per validator; this is not a finalized worldwide participation limit.

Validator updates take effect after the consensus engine's two-block delay. The application stores the expected signing sets by effective height and validates the preceding commit against the set for the block actually signed. It does not substitute newly selected validators for prior signers.

The staking bank adapter routes any upstream burn request from bonded or unbonded pools into quarantine. End-to-end evidence handling, double-sign tombstones, the offline window, suspension and signed reactivation are not wired in this runtime. Blocks containing misconduct evidence fail closed. This limitation prevents production use: it is not a completed penalty system or a safe response to adversarial evidence.

## Contribution rewards

GOD does not issue block inflation. G allocations are derived internally from the last commit delivered by the local consensus client. GodCometBFT verifies the real signatures; ABCI vote flags alone are not cryptographic proof. The runtime has no remote ABCI socket through which a participant can supply forged contribution flags or reward amounts.

An eligible delegation snapshot is taken before transactions in the block being signed. Later stake changes cannot retroactively earn that block's reward. A signing validator receives its share according to the consensus voting power expected for that height. Its 10 percent operator portion is separated first; the remainder is divided among eligible self-delegation and other delegation shares. The operator's self-stake participates in that remainder. Absent or nil votes receive no allocation, and their unused shares are not redistributed.

`GPerSignedBlock` must be explicitly provided for synthetic testing; no production value is selected. The allocation budget is limited to the remaining 10,000 G UTC-day ceiling. All divisions round down, and rounding dust remains unissued. The fast network fixture deliberately uses a test budget, not a published emission schedule. A roughly six-second production cadence would still require an approved, measured per-block rule.

Only the preceding block is considered. A signature from a prior UTC day receives no catch-up allocation in the new day. Pending earned G settles through the existing day-boundary mechanism; it is not immediately spendable or redeemable. Current GOD fees also remain pending until that boundary. The voluntary redemption rule and G's other ecosystem uses are unchanged.

## Isolation and lifecycle

Configuration and synthetic genesis must select prototype mode. There is no backed genesis pathway or mainnet activation flag. Genesis is bounded, strictly decoded and deterministically ordered; account balances plus restricted reserve equal the fixed GOD supply. Initial stake is transferred out of those synthetic balances, not created in addition to supply.

The ABCI adapter bounds transaction bytes, count and requested gas in proposals and finalized blocks. It enforces sequential heights, monotonic time and commit order. Internal execution failures poison the instance so a recovered panic cannot authorize a partial commit. Snapshots read explicit committed versions, not the SDK's pre-commit working state. Unsupported vote extensions and state-sync restore are rejected.

The pinned execution configuration is process-global and sealed. One runtime is allowed per process; a second constructor fails before modifying it. Independent validators require separate processes. Persistent restart and consensus replay of this combined runtime still require verification.

`StartLocal` accepts loopback peers only. It disables peer discovery, external RPC and gRPC, profiling, metrics and transaction indexing. Consensus databases are in memory; test-owned WAL and peer-book files remain in private temporary directories. Signers are caller-provided, and only private fixtures use ephemeral mock signers. A durable signer and recoverable node storage are required before an operator release. No populated address, key, genesis or endpoint is supplied in source.

The diagnostic reports `local-consensus-execution-prototype`, `localNodePrototype: true`, `nodeReady: false` and `realAssets: false`. Its financial and startup commands remain disabled. The bridge prototype remains outside this runtime and is not connected to RH.

## Local evidence and remaining gates

Local tests exercise all three enabled Ethereum transaction families, signature replay, revert fees, protected module rejection, GOD transfers in both address orders, contract deployment and storage, self-destruction quarantine, signed validator creation, staking delegation and delayed exit, supply conservation and no halt catch-up. Direct application fixtures contain synthetic commit flags and are not signature evidence. The separate four-process network test independently verifies stored cryptographic commits, propagates a signed contract deployment over P2P and compares persisted code and storage across processes. After one validator stops, the other three continue; the absent validator's G does not increase and the actual signers' G does.

Before production, verify all enabled EVM transaction families and adversarial failures, complete nonburning penalties, approve emission rules, test durable signer and crash recovery, integrate authenticated bridge transactions and verified backing, implement governance, and measure resource use on ordinary computers. Long-period settlement scans, delegation scans and retained tombstones still need performance and growth limits. Local processes on one computer are not geographically independent validators or a demonstrated decentralized launch.
