# Synthetic GOD Chain wallets

This exact public core snapshot includes the encrypted terminal signer and the
unsigned local companion. Separately maintained web and downloadable Chrome
alpha 0.3.9 operate on the authorized synthetic pilot; their browser source,
gateways, deployment scripts, packages and populated network settings are not
included here. Public build commands do not install or activate those services.
All test assets have no monetary value or guaranteed mainnet conversion.

The connected alpha offers a locked-wallet lookup of one configured operator's
committed staking record. Selecting a fresh bonded, non-suspended record only
fills an unsigned delegation form; it does not start a signer or resolve an
unknown transaction. The configured list is not a complete validator index,
and the record is not a signing, uptime, ownership or location proof.

The locked-wallet registration directory also exposes at most eight current
records per page in raw operator-key order. Exclusive cursors bind the same
latest commit; a changed tip or rejected read clears the page and requires a
new first request. It does not merge snapshots, retry automatically, start a
signer or clear unknown-TX guards. Live desktop/mobile/downloaded Chrome views
checked all four current records. This directory is not historical indexing,
the actual consensus signing set, an uptime ranking or independent ownership
or geographic proof. The configured lookup remains a separate unsigned helper.

## Local-only recovery candidate

The separate opt-in Chrome 0.4.0 candidate is not installed or activated by
this core upload. Explicit website consent shares only a public account, never
a signature. Locally reviewed GOD, restricted NFT and six native actions share
a same-profile durable unknown-attempt guard. Original public review and TX ID
are retained before submission, without a vault, password or signed bytes.
Reload and browser restart restore the review for manual result checking;
missing/mismatched results block new signatures. An expired signed redemption
does not erase its unresolved record. Verified backup re-encryption preserves
the same account and coordinator. Nothing is automatically resent.

Storage deletion, extension removal or another device can bypass this local
guard and do not prove transaction rejection. Save known TX IDs separately.
Local recovery-file code can restore a previously saved signed unknown guard
into an empty matching account journal without signing, submission or overwrite.
It is public lookup metadata, not payment or signature proof, and cannot recover
later missing attempts. One plain GOD browser-file recovery run passed, but
earlier reopened Chromium downloads crashed and the gate is not reliably
accepted. NFT/native file handling has state-machine coverage only. Physical devices,
comprehensive storage-loss recovery, source/distribution review and
independent review remain incomplete. The current distribution command refuses
the candidate, and the public 0.3.9 package is unchanged. See the developer
checks and limitations in [VERIFICATION.md](VERIFICATION.md#local-only-wallet-recovery-candidate).

## Included terminal signer

Use the checksum-bound build and a macOS or Linux terminal. Windows private-file
operations remain disabled pending owner-only ACL acceptance. Keep the private
wallet directory outside node/web workspaces with mode 0700. Wallet/request and
signed files use mode 0600; existing output files are never replaced. Configure
absolute private paths and an independently reviewed complete bundle digest:

```sh
./build/godd wallet create --wallet "$TEST_WALLET_FILE" \
  --bundle "$TEST_BUNDLE_FILE" --expected-bundle "$TEST_BUNDLE_SHA256"

./build/godd wallet address --wallet "$TEST_WALLET_FILE" \
  --bundle "$TEST_BUNDLE_FILE" --expected-bundle "$TEST_BUNDLE_SHA256" \
  --output "$TEST_ADDRESS_FILE"

./build/godd wallet sign --wallet "$TEST_WALLET_FILE" \
  --bundle "$TEST_BUNDLE_FILE" --expected-bundle "$TEST_BUNDLE_SHA256" \
  --request "$TEST_REQUEST_FILE" --output "$TEST_SIGNED_FILE"
```

The terminal hides the unique password and requests it twice for creation.
Passwords require 12 to 256 UTF-8 bytes without leading/trailing whitespace,
newline or NUL. Password arguments, environment values, files and piped input
are refused. Back up the encrypted file and keep its password separately;
there is no password reset, seed phrase or guaranteed recovery of lost data.
The encrypted account binds the complete bundle and cannot be silently rebound.

Canonical lowercase `god1` and compatible `0x` encodings identify the same
twenty-byte account and shared sequence, not separate balances or a bridge.
Use fresh committed state before review and avoid concurrent signing. Review
the chain, bundle, sender, recipient/operator, exact amount, account number,
sequence, gas and offered fee. Unlocking is not consent: type `SIGN` only for
the displayed intent. Redemption also binds minimum output and deadline.

The six native scopes are delegation, start-unbonding, settled-G claim,
spendable-G transfer, voluntary GOD pool contribution and G redemption. Plain
GOD transfers use a chain-bound compatible signature with 21,000 gas and no
calldata. Arbitrary contract calls and real-asset operations are not supported.

The distinct operator-only `wallet sign-registration` command reviews and signs
a private synthetic candidate request. It validates the node-key proof and exact
wallet owner on an explicitly proof-enabled synthetic bundle, then displays
the consensus public key, chain/genesis, self-stake,
minimum stake, commission, enforced consensus-proof policy, fresh account
metadata and fee before hidden `SIGN`. A compact proof travels on-chain inside
the standard staking transaction and is checked by the proof-enabled chain.
It uses the encrypted wallet in memory and writes a new private transaction;
there is no plaintext-key export, network access or automatic submission. This
does not expand `wallet sign`, web/Chrome actions or the public six-action gateway.
See [candidate setup](TESTNET.md#fresh-participant-and-validator-candidate-workflow).

The terminal signer never contacts RPC. Anyone holding signed bytes can submit
them; keep signed files private and use only a separately reviewed synthetic
submission workflow. Admission is not inclusion. Match the committed receipt
and current account/delegation separately. Unknown results are not rejection
and do not permit a replacement signature. Exact definite non-admission needs
fresh state, review and explicit approval, never automatic resend. Public RPC
remains read-only even though a privately configured node has submission APIs.

## Separately deployed participant alpha

The connected web/Chrome alpha supports disposable creation, encrypted backup
and recovery, automatic bounded funding, plain GOD transfers, restricted test
NFT mint/send and six explicitly reviewed native actions. Signing uses local
disposable workers, not server-held participant keys. No registration or human
review is required for the faucet; fixed quotas and available funds still apply.

Native status and submission have independent IP cooldowns and a combined
global budget. Definite pre-admission failure is distinct from unknown or
duplicate attempted results. Earlier unknown IDs retain their conflicting-
signature guards. A network-bound native TX ID file enables locked read-only
recovery without proving ownership or unlocking the account.

The G readiness view separates spendable, settled unclaimed and pending earned
G, as well as settled/pending pool and supply. Pending G is not claimable or
redeemable. Its minimum-one-G indicative output uses only settled liquidity and
outstanding G with integer flooring; empty or zero-output liquidity has no
positive quote. Every redemption still requires fresh state, signed minimum
output/deadline and explicit consent. A read burns no G and signs nothing.
The next UTC boundary comes from committed block time and is not a settlement,
reward or payout guarantee. Live web/mobile/Chrome values matched exact state.

Public delegation/start-unbonding receipts and a real pending one-GOD entry
were checked, including locked web/Chrome recovery. The progress view shows
committed locked balance, scheduled UTC time and holds. A successful initiation,
time-reached entry or empty query is not proof of payout. The existing 21-day
delay is unchanged. See [VERIFICATION.md](VERIFICATION.md) and
[ROADMAP.md](ROADMAP.md) for remaining acceptance gates.

## Security and remaining limits

The terminal format uses authenticated AES-GCM and fixed scrypt parameters;
strict bounded parsing, private permissions and exclusive creation fail closed.
Encryption does not protect a compromised device, malware, weak passwords or
screen capture; buffer clearing is not guaranteed secure erasure.

Complete delayed withdrawal and positive settled-G acceptance, physical target
devices, sustained capacity, HD/hardware accounts, production recovery,
website-provider permissions and independent security review remain incomplete.
The Chrome alpha uses manual unpacked installation, not Store publication or
automatic updates. No RH bridge, mainnet asset conversion or real payment is
enabled. Build/publication does not authorize a chain launch or real assets.
