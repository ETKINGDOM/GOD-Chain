# GOD Chain Whitepaper Draft

Planning edition dated October 3 2026

Project account ETKINGDOM

## Purpose and status

ETERNAL KINGDOM is a faith-centered world for prayer, confession, praise, fellowship and remembrance. GOD Chain is the proposed independent blockchain that will support the world and let independent participants help preserve its records and operation.

The direction is to begin with GODCOIN on RH, develop GOD Chain, bridge the same economic asset one for one, and progressively enable the application to operate on the independent network. The project is intended for a global community, with English as the default language and support for original-language faith expressions.

This is a planning draft, not a launch announcement. It records the selected direction and proposed initial parameters. It does not establish that the independent mainnet, staking rewards or bridge are implemented, audited or operating. The founding whitepaper remains a separate, unchanged document. No launch date, token price, guaranteed yield or perpetual availability is promised.

## Mission and application

The network exists to serve the eternal heavenly world, not to measure faith through wealth or computer resources. Prayer, confession and praise remain expressions offered by people; a transaction does not certify divine forgiveness, spiritual rank or the truth of a revelation.

The application will encrypt private faith text on the user's device before submission. The chain can store that ciphertext as application data. The application is responsible for encryption, decryption, consent and any key backup. Publicly readable expressions require an explicit user choice. Private plaintext and decryption keys must never enter public chat, telemetry or operational logs.

Message size and fees will be measured in encoded bytes, including encryption overhead. Transaction metadata can remain public even when the words are encrypted. Onchain persistence is not a guarantee of anonymity, recoverable lost keys or preservation for all future time.

The independent chain is one part of the decentralization roadmap. Community hosting, world assets, social services, indexes and client distribution require their own continuity work; moving transactions to a new chain does not automatically decentralize the entire application.

## Selected network architecture

The project uses the following exact development and product names.

| Component | Responsibility |
| --- | --- |
| God EVM | Smart contract execution, compatible wallet and application transaction interfaces |
| God SDK | Application state, staking, G accounting, fee routing, redemption and governance modules |
| GodCometBFT | Validator communication, block agreement and consensus finality |

The selected model combines a fixed GOD economic supply and community-operated infrastructure with an EVM-compatible execution environment and stake-weighted consensus. It is not proof-of-work mining, and it does not give every computer an equal consensus vote.

God branding identifies this project's integration and network. Third-party technology is not presented as independently invented. Required licenses, attribution and necessary dependency identifiers are retained. Compatible maintained releases must be selected and pinned before implementation; branding does not alter their security properties.

## GODCOIN and the supply invariant

GODCOIN is the planned RH asset, with a fixed economic supply of 1,000,000,000 GOD. RH transactions pay native gas in ETH. GOD Chain transactions will pay gas in native GOD. RH's documented gas asset is ETH; this fact does not verify this project's token issuance or integration. [RH network documentation](https://docs.robinhood.com/chain/)

The bridge must not create a second freely circulating billion GOD. Each unit released for use on GOD Chain must correspond to one unit locked on RH. Returning that unit to RH removes it from use on GOD Chain before the RH unit becomes spendable again. GOD held in staking, fee pools or participant accounts remains part of the same bridged economic supply.

The proposed native implementation allocates 1,000,000,000 GOD to a restricted protocol bridge reserve at genesis. Those reserve units are not freely spendable, reward funding or developer inventory. They become usable only against verified RH escrow backing. Raw balances recorded on both chains must not be added together and presented as circulating supply.

Initial validator balances must also be backed. Verified RH genesis deposits must be reconciled into the initial state, with matching amounts removed from the restricted reserve. The network may not bootstrap security by awarding unbacked native GOD.

Before any real-asset release, implementation must verify the actual RH token supply, mint permissions, supported transfer behavior, escrow balances and native reserve accounting. Initial token allocation, treasury ownership, any sale and any vesting schedule are not specified in this draft.

## Participation and decentralization

People may delegate GOD, operate an independent full node, or seek an active validator position. A full node verifies and relays network data. An active validator additionally signs consensus votes. Delegating does not require the participant to keep a personal computer online; running a full node alone does not automatically earn G.

Active validators are selected by total self-delegated and delegated GOD, subject to the proposed active-set limit. Registration is not a developer whitelist. Multiple machines or accounts do not create additional flat rewards; rewards depend on eligible stake and valid contribution.

The launch objective is at least seven genuinely independent validator operators, with different controlling parties and failure domains. Seven machines controlled by one person are not seven independent operators. The community must monitor effective stake concentration, related operators and correlated infrastructure outages.

Consensus requires more than two thirds of active voting power to agree. One third or more can halt progress by withholding votes. Independence and stake distribution matter more than device count alone. [Consensus specification](https://github.com/cometbft/cometbft/blob/main/spec/consensus/consensus.md)

## Proposed staking and G rewards

These values are the initial design baseline, not measured production-safe settings or infrastructure defaults.

| Parameter | Proposed initial value |
| --- | --- |
| Minimum delegation | 1 GOD |
| Minimum validator self stake | 1,000 GOD |
| Maximum active validators | 32 |
| Independent operator launch objective | At least 7 |
| Target block interval | Approximately 6 seconds |
| Network-wide G issuance ceiling | 10,000 G per UTC day |
| Reward commission | 10 percent to the operator and 90 percent shared by eligible self stake and delegations |
| GOD unbonding period | 21 days |

G is a distinct ecosystem reward unit. It is not newly issued GOD, a claim to return staked principal, or a fixed-value substitute for GOD. Its first planned utility is proportional redemption against the GOD reward pool. Further ecosystem uses require separate specifications.

The daily ceiling applies to the whole network, not each node. Actual G issuance will be limited by valid completed block contribution, with no catch-up issuance for halted periods. Distribution will use eligible bonded stake and verified signing participation, rather than device counts or an application heartbeat. The exact block-based issuance calculation and rounding rules must be specified and tested before the ceiling can become an operational rule.

Self stake participates in the shared 90 percent on the same basis as eligible delegations. Unbonding amounts stop earning new G, while already settled G remains valid for redemption. Reward calculations must account for changes in stake during a day rather than relying only on the closing balance.

## Offline behavior and penalties

Brief offline periods lose the corresponding G reward but do not slash GOD principal. The proposed monitoring window is 10,000 expected signing opportunities. More than 1,000 missed signatures results in validator suspension for at least one hour. A suspended operator must resynchronize and request reactivation.

Double signing incurs a proposed penalty of 5 percent of the applicable infraction-related stake and permanent exclusion of that validator identity. Relevant delegators can also bear the penalty, including applicable stake still in an unbonding or redelegation exposure period.

To preserve the chosen noninflationary, nonburning GOD accounting, penalized GOD moves to a segregated protocol quarantine account. It is not sent to the G redemption pool. This routing requires custom implementation: the reviewed upstream staking code burns slashed coins. Unbonding, redelegation, fee handling and governance deposit paths must all be reviewed for unintended GOD destruction or issuance. [Staking penalty implementation](https://raw.githubusercontent.com/cosmos/cosmos-sdk/v0.53.0/x/staking/keeper/slash.go)

No release or distribution policy for quarantined GOD is specified here. It must remain unavailable unless a separate reviewed policy is adopted.

## Gas revenue and the GOD reward pool

The intended network rule routes paid GOD gas fees into the protocol reward system instead of minting new GOD for rewards. Collected fees await daily settlement before they become available for redemption. Implementation must cover all relevant base-fee and priority-fee paths and prevent unintended fee burns or double distribution.

The spendable redemption pool is separate from validator stake, unbonding balances, RH escrow, the native bridge reserve and quarantined penalties. None of those balances may be counted as reward liquidity or used to pay G holders.

The initial redemption pool is 0 GOD. Early funding comes only from actual GOD fees and voluntary contributions of existing, correctly backed GOD. There is no assumed treasury deposit or guaranteed external subsidy. G can accrue under the reward rules while the pool is empty, but cannot then be redeemed for GOD.

Running nodes still costs electricity, connectivity, storage and maintenance. Low initial transaction demand can leave rewards below operating costs. Community participation may initially be voluntary; a fixed token supply does not make infrastructure free.

## Floating redemption and G destruction

Let P be the available, settled GOD redemption pool. Let T be all settled, valid and outstanding G, including confirmed reward entitlements not yet claimed. Let g be the G a participant offers to redeem, where g is no greater than their eligible balance and T.

The rule is:

`GOD payout = floor(P * g / T)`

Amounts are calculated in integer smallest units. A successful redemption atomically pays GOD, destroys the redeemed G and reduces both pool assets and outstanding G. If the pool, outstanding total or calculated payout is zero, the operation is rejected without destroying the participant's G. The minimum proposed redemption is 1 G, with normal transaction gas and no separate protocol redemption charge.

For example, a pool of 10,000 GOD and 100,000 outstanding G gives a pre-gas payout of 100 GOD for 1,000 G. After redemption the pool holds 9,900 GOD and 99,000 G remains. Redemption alone therefore does not materially change the ratio, apart from rounding. New GOD income increases the backing per G; additional G issuance can dilute it.

GOD paid out is transferred, not burned. Only redeemed G is destroyed. Redemption fees paid as transaction gas join the pending fee account for a later settlement, not the payout pool used by that same operation.

Daily settlement occurs at the first valid block at or after 00:00 UTC. The prior day's collected fees and earned G are brought into the available pool and outstanding-G denominator in one system transition. Confirmed unclaimed G must never be omitted from that denominator. Settled G can be redeemed between settlements.

G balances, issuance, entitlements, transfers, destruction and redemption must be independently verifiable onchain. Applications can support offchain ecosystem activity, but an operator-controlled database cannot arbitrarily create valid redemption rights. G supply grows through reward issuance and shrinks through redemption; no fixed lifetime G cap has been selected.

This mechanism defines proportional access to available assets, not a fixed exchange price or promised annual return. Fee-demand assumptions, long periods of low income, reward dilution and adversarial self-generated transactions require economic testing before deployment.

## RH bridge design and initial limits

The first proposed bridge uses lock and release in both directions. RH escrow and the native restricted reserve must be independently reconcilable. Messages require domain separation, unique sequence identifiers and single execution, with finality checks on the source chain.

The initial design requires five approvals from seven independent bridge signers. This is a federated bridge with additional signer trust, not a fully trustless bridge. A bridge's verification model introduces security assumptions beyond the two connected chains. [Bridge verification models](https://ethereum.org/developers/docs/bridges/)

| Control | Proposed initial value |
| --- | --- |
| Current real-asset activation | Disabled |
| Bridge signer threshold | 5 of 7 independent signers |
| Future initial RH escrow exposure limit | 10,000 GOD |
| Maximum single transfer | 1,000 GOD |
| GOD Chain to RH public withdrawal delay | 24 hours |
| Maximum GOD Chain to RH outflow | 1,000 GOD per rolling 24 hours |

These nominal limits are proposed risk controls, not proven safe financial exposure. They require implementation, value-aware review and explicit authorization before activation. Queue ordering, rate-limit enforcement, pause behavior and signer replacement must be specified before any real assets enter the bridge.

Source confirmation must not rely only on a fast RH sequencer receipt or one trusted RPC. Each signer must verify the required finalized source evidence. Exact RH confirmation and incident-handling rules remain implementation work.

A later proof-verified bridge may reduce signer trust, but is a separate development project. Choosing an EVM-compatible chain does not supply such a bridge automatically. No bridge key, wallet address, contract address or operational endpoint is included in this draft.

## Governance and upgrades

The developer may write code and submit proposals, but should not have unilateral authority to move escrow, spend the reward pool, create GOD or secretly upgrade the network. Staked GOD is the proposed governance weight. Delegators can override their validator's inherited vote.

| Governance action | Proposed initial rule |
| --- | --- |
| Routine proposal voting | 7 days |
| Routine turnout requirement | At least 67 percent of total bonded voting power |
| Routine approval | More than 67 percent yes among nonabstaining votes |
| Notice before a passed routine software upgrade | At least 35 days |
| Emergency security vote | 24 hours, with the same turnout requirement |
| Emergency approval | More than 80 percent yes among nonabstaining votes |
| Emergency bridge pause | 5 of 7 bridge signers |

The 35-day routine notice is intended to provide time for the proposed 21-day unbonding period, withdrawal delay and drainage of the initially capped bridge under its daily outflow limit. It is not an unconditional withdrawal guarantee: chain halts, pauses, source-chain incidents or insufficient cooperation can delay exit. Larger future bridge limits require a new exit-capacity analysis.

Emergency proposals require a public patch and activation height. They do not silently force independent operators to install software. A bridge pause may stop transfers but must not authorize asset movement; resumption and control changes require the reviewed governance path. Independent bridge-control enforcement must be specified on both chains.

High participation thresholds can also prevent necessary action when participation is low. Concentrated stake can concentrate governance power even with many devices. The community must test upgrade failure, disagreement, signer unavailability and emergency recovery before treating these rules as effective controls.

## Ordinary computer operating target

The objective is to support ordinary desktop computers without a GPU or specialist mining hardware. The following is an acceptance target to test, not a measured minimum specification.

| Resource | Initial test target |
| --- | --- |
| CPU | Modern 4-core processor |
| Memory | 16 GB RAM |
| Storage | 500 GB available SSD space |
| Network | Stable 10 Mbps upload and download |
| Initial block gas limit | 2,000,000 gas |

Active validators need reliable continuous operation. Household equipment can face power loss, restricted connectivity or changing network conditions. Eligibility is distinct from guaranteed validator selection or profitable operation.

Benchmarking must cover sustained full blocks, encrypted message workloads, state growth, first synchronization, recovery after extended offline periods and ordinary home connections. Lower-memory systems require separate testing. Pruning historical blocks does not eliminate live application state or provide unlimited permanent storage.

## Development roadmap

Progression is evidence-based, not tied to promised calendar dates.

| Stage | Work and evidence required |
| --- | --- |
| 1 RH application integration | Verify the actual GODCOIN issuance and supply controls; keep wallet consent, faithful record handling and truthful capability states |
| 2 Local GOD Chain prototype | Build God EVM, God SDK and GodCometBFT integration; specify integer units, issuance rules, fee routing, staking and G accounting |
| 3 Independent computer test network | Run several separately controlled computers; measure hardware requirements, signing reliability, consensus behavior and recovery |
| 4 Economic and bridge testing | Verify supply conservation, escrow and reserve reconciliation, G entitlements, atomic redemption, penalties, queues, replay protection and adversarial economics |
| 5 Explicitly authorized limited mainnet | Reconcile backed genesis stakes, establish independent operators and signers, verify governance controls, then enable only reviewed real-asset limits |
| 6 Application migration and continuity | Enable the application on GOD Chain; preserve RH records and receipts; document community hosting, independent indexes and recovery |

Users are not automatically moved, and historical RH records are not rewritten as GOD Chain transactions. Cross-network applications must identify the actual network and confirmation state of each record. The eventual transition to GOD Chain must retain access to existing faith records.

## Review and release requirements

The project will begin with self-review, reproducible tests and open community maintenance. Paid external auditing is not presumed as a prerequisite for beginning development. Internal tests and community review must not be described as an independent security audit or a guarantee of maximum safety.

Release checks must verify the fixed-supply invariant, actual backing, all mint and burn paths, stake changes, penalties, daily G settlement, rounding, redemption atomicity and behavior through chain halts. Bridge checks must include duplicate messages, conflicting source evidence, compromised signers, pauses and stuck withdrawals. Performance checks must validate the ordinary-computer target.

The consensus stack does not by itself provide the proposed G rewards, proportional redemption, nonburning penalty routing or RH bridge. Those are custom protocol components with their own implementation and review burden.

Public source and its reachable history must contain no credentials, keys, test addresses, populated wallet addresses or token contract addresses. Required third-party notices remain intact. Public publication, deployment and real-money activation require separate explicit authorization.

## Decisions and remaining specification work

The selected direction is the faith-centered global application, God EVM plus God SDK plus GodCometBFT, fixed GOD economic supply, one-for-one RH bridging, GOD gas, staking-related G rewards, fee-funded proportional GOD redemption and destruction of redeemed G.

The numerical settings in this draft are proposed initial parameters. Before launch, the project must finalize compatible component versions, exact reward and denomination calculations, transfer and rounding rules, genesis reconciliation, fee implementation, bridge proof and queue rules, governance execution, operator and signer independence, message-size limits, measured hardware requirements and maintenance funding. Token allocation and any treasury or sale policy remain unspecified.

ETERNAL KINGDOM's ambition is an enduring world of faith maintained by its community. GOD Chain is the infrastructure plan for pursuing that ambition, with transparent limits and responsibility rather than a claim that permanence, decentralization or financial returns have already been achieved.
