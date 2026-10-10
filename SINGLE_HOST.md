# GOD Chain single-host operating plan

The selected initial deployment is one founder-operated physical machine with
multiple isolated services. The stack remains God EVM + God SDK + GodCometBFT.
Additional machines and independent operators are a later expansion, not a
requirement to continue the local single-host candidate. This selects an
operating topology, not production economics, mainnet launch or permission to
change the existing pilot. Real assets and RH bridge activation remain gated.

## Service layout

| Role | Single-host arrangement | Boundary to preserve |
| --- | --- | --- |
| Consensus | Four validator processes, each with its own identity, data directory, P2P port and loopback RPC port | Never share a running signer/data directory, duplicate a validator identity or remove the quorum rule |
| Public reads | One separate non-signing observer process on the same host | Public clients use a restricted HTTPS gateway, not direct validator RPC |
| Wallet and explorer website | Static interface/companion behind HTTPS | Wallet secrets and signing remain on the participant's device |
| Test funding | Separate synthetic faucet with restricted operational storage | No real GOD, reserve unlocking or guaranteed conversion; preserve known/unknown claim IDs |
| Transaction submission | Separate keyless plain-transfer, restricted NFT and native-action boundaries | Sharing the host must not merge allowlists or enable arbitrary calls/registration |
| Retained history | Separate bounded address and collection workers plus read-only snapshot readers | Separate working/published files; a request cannot launch backfill or repair storage |
| Supervision and diagnostics | Reviewed service units, finite observation and operator incident handling | Never auto-reset signing progress or treat a failing check as restart authorization |

All services compete for the same CPU, RAM, disk and network. Four validator
stores and an observer store replicate data on the same volume; history,
backups and staging require additional space. Do not size the host as a single
node or present short fixtures as a minimum ordinary-computer specification.

Keep separate owner-only directories and nonoverlapping ports. Use unprivileged
service users and role-appropriate read/write restrictions, with executables
outside writable data directories. Existing management access does not require
adding an administrator. The host administrator still controls all services;
separate Unix users and processes do not decentralize consensus.

Only reviewed HTTPS ingress is intended for public application access. Validator
RPC and internal gateways stay on loopback. Remote participant peering needs a
separate firewall/admission review and is not enabled by publishing the website.
Exact hosts/origins, deadlines and abuse limits still apply on a shared host.

## Fault and capacity acceptance

Four equal validator powers require more than two-thirds of total power to
commit. Three can continue; two cannot. A non-signing observer adds no voting
power. This rule must not be lowered to conceal a single-host outage.

The private `make test-single-host-recovery` fixture is excluded from this public
source snapshot. It uses persistent disposable nodes and actual synthetic consensus. It checks one-validator process loss, loss of quorum after
stopping a second validator, retained identities/progress, both graceful and
forced-process-exit whole-group restart, lagging-node catch-up, matching
committed blocks/balances and
a non-signing observer. All processes run on one computer. This is not physical
power loss, damaged-disk, Linux reboot or independent-machine acceptance.

The bounded `godd testnet read-load` helper in [DEPLOYMENT.md](DEPLOYMENT.md)
checks concurrent liveness/network reads, deadlines, consistent snapshots and
short height progress. Its loopback fixtures and disposable local consensus
test do not establish aggregate host capacity or wallet/write-load acceptance.
Do not use its small latency series as a concurrent-user or throughput promise.

`make test-mixed-wallet-load` adds a controlled private correctness drill for
concurrent claim/transfer/NFT requests, delayed reads, actual committed results,
lost-acknowledgment recovery and observer restart. It keeps existing quotas and
uses a three-connection relay, not the public ingress. Twelve mutation requests
start together; four reads follow after 600 milliseconds. It does not complete
intended-host aggregate load acceptance, native-action concurrency, physical
browser testing or pre-inclusion keyless-gateway crash durability. See
[DEPLOYMENT.md](DEPLOYMENT.md#bounded-local-mixed-wallet-acceptance).

`make test-native-concurrency` separately checks eight identical native requests,
genuine pre-inclusion unknown recovery through a checksum-pinned offline attempt
copy, one-time committed donation/undelegation and three no-attempt zero-G
rejections. Two stopped validators cause an actual quorum pause; the fixture
never advances the settlement clock or shortens the 21-day wait. A malformed
logical record is injected only into a disposable closed copy, not physical
pages or node storage. These are bounded native correctness/recovery checks,
not full mixed intended-host load, positive elapsed-day G settlement, power-loss
durability or public acceptance. See
[DEPLOYMENT.md](DEPLOYMENT.md#bounded-local-native-concurrency-and-unknown-recovery).

Before a proposed single-host release, measure and exercise the intended host
with worthless test assets:

The finite [`host-observe` helper](DEPLOYMENT.md#finite-linux-resource-observation)
adds explicit same-user process RSS/descriptor aggregates, procfs CPU ratios,
estimated available memory and filesystem space. Its portable fault checks and
private Linux compilation do not complete host acceptance. Separate service
users require separate observations; approximate RSS and a short procfs series
do not establish all-service or cgroup headroom, sustained load or a participant
limit. Keep the intended-host measurements below as release gates.

The private [stopped-observer cold-copy drill](DEPLOYMENT.md#private-stopped-observer-cold-copy-drill)
checks a new retained copy of a genuinely closed synthetic observer workspace,
committed state/receipt retention, subsequent catch-up and unchanged nonsigning
identity. It refuses validator copies and does not provide an operator restore
command. Independent private custody, authenticated checkpoints, physical
durability, validator fencing and safe upgrades remain unaccepted.

1. Total/per-service peak CPU and RAM during catch-up and normal public traffic.
   Choose host headroom and service limits from measurements, not service count.
2. Aggregate disk growth, attempt/history capacity, file handles and bandwidth.
   Refuse before exhaustion without deleting unknown submissions or claiming
   partial retained history is complete.
3. Bounded concurrent wallet reads, faucet claims, transfers, NFT and native
   actions, including quotas, committed results and recovery IDs.
4. Process faults, observer isolation, quorum loss and whole-host reboot.
   Stop writers before reopening their workspaces; retain signer identity/state.
5. Sustained actual UTC-day G settlement and unchanged waiting-period payout.
   Synthetic timestamps are unit-test inputs, not elapsed time.
6. Reviewed immutable update, migration and rollback drills. Software rollback
   must not roll back signing progress, chain state or unresolved operation IDs.

An observer on this host cannot notify anyone after the whole host fails.
Same-disk snapshots cannot recover a lost disk. Review an independently retained
private backup/checkpoint and an out-of-host incident signal before claiming
disaster recovery or reliable outage alerting. These do not require a second
running validator, but destination/access/custody need separate approval. Never
publish validator/wallet keys or casually restore an older signer state.

## Release stages

| Stage | Establishes | Does not establish |
| --- | --- | --- |
| Local single-host candidate | Code, bounded fixtures and process recovery on one computer | Intended Linux capacity, public uptime or production approval |
| Accepted single-host public test | Reviewed target-host and participant checks with worthless assets | Independent validation, multi-region resilience or real backing |
| Proposed founder-operated production | Separately reviewed economics, funding, recovery, governance and applicable gates | Decentralized control or authorization to activate real assets |
| Later distributed participation | External operator-owned nodes after actual peering/admission acceptance | Independence merely from more processes, keys, IP labels or region names |

Do not convert test balances into real GOD or bypass synthetic-only startup
checks. A live RH bridge still needs authenticated source/finality evidence,
reviewed custody and one-for-one backing, not just a token contract setting.
Founder control and the shared-host single point of failure must remain explicit
in participant release information. A whole-host outage stops all local
validators and application services; this layout has no cross-machine failover.

Cross-machine/geographic acceptance moves to the later distributed stage. It is
not relabeled as passed and must pass before advertising independent validation
or multiple physical locations. Production gates remain in
[ROADMAP.md](ROADMAP.md#production-transition-gates); private staging, budgets
and templates are in [DEPLOYMENT.md](DEPLOYMENT.md).
