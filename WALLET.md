# Synthetic GOD Chain wallets

This exact public core snapshot includes the encrypted terminal signer and the
unsigned local companion. Separately maintained web and downloadable Chrome
alpha 0.3.6 operate on the authorized synthetic pilot; their browser source,
gateways, deployment scripts, packages and populated network settings are not
included here. Public build commands do not install or activate those services.
All test assets have no monetary value or guaranteed mainnet conversion.

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
