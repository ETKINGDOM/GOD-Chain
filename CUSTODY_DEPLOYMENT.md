# Private source custody deployment preparation

This is an **offline unsigned preparation workflow**, not a deployed bridge or
permission to spend ETH. It creates no provider, signer, nonce reservation,
source account, service or production route. Publishing source enables none of
those operations. The existing contract remains a limited custody prototype,
not an independently audited or production-approved contract.

## Exact source and compiler

Only the reviewed `contracts/GodBridgeEscrow.sol` bytes and the project's
existing pinned Solidity 0.8.37 compiler are accepted. Source changes require a
new source review, not a caller-selected replacement checksum. Compilation uses
the fixed source label, optimizer enabled with 200 runs, no via-IR, Cancun and
metadata bytecode hash `none`. Imports, links and alternate targets/settings are
not selectable. Both the explicitly supplied local Node executable and the
self-contained compiler file are independently hash-checked before and after
execution. Final symlinks and shared-writable files are refused; select the
physical executable, not a package-manager symlink. No automatic download or
PATH fallback exists. Compiler execution is bounded and emits no diagnostics
into the public report. A quiescent reviewed host is required; this is not an
OS security sandbox or proof of toolchain provenance.

## Explicit private request

`rh-custody-deployment-template` prints blank slots only. The request requires:

- Token contract, source chain ID, source/native domain names and exactly seven
  nonzero, unique signing members in ascending raw-address order. No member or
  wallet is generated. Seven identifiers do not demonstrate seven independent
  operators or possession of their keys.
- Transaction sender, nonce, Gas limit, maximum fee per Gas, priority fee per Gas
  and maximum execution-fee cost. Every numeric value is a canonical decimal
  string, including nonce zero; no fees, nonce or sender are guessed. The Gas
  limit retains the current prototype's bounded admission policy. The supplied
  budget must cover `Gas limit × maximum fee per Gas`; tip cannot exceed fee.

Unknown fields, duplicate/escaped duplicate keys, case aliases, nulls, trailing
JSON, invalid names, unordered/repeated members and out-of-range numbers fail
closed. Constructor ABI is fixed to the reviewed contract. The result binds the
exact creation bytecode and constructor parameters to a type-2 **contract
creation** transaction with no destination, zero value, no access list or
authorizations, and zero signature fields. The CREATE address is calculated from
the explicit sender and nonce. It is only a prediction, not an existing custody
instance or a trustworthy asset binding.

Init code is bounded to 48 KiB. The Cancun creation intrinsic Gas floor includes
creation and init-code-word charges, rather than reusing a plain-transfer floor.
Passing that floor is **not enough Gas to execute the constructor**. No source
Gas schedule, live estimate, additional source-chain fee, current nonce,
deployment-account balance, collision availability or fee market is verified.
Constructor checks on token code, decimals, fixed supply and initially empty
custody balance still run only when the contract is actually executed.

## Commands and retained packet

All commands below share these eight ordered arguments:

```text
<absolute-reviewed-source> <retained-source-SHA256>
<absolute-private-request> <retained-request-SHA256>
<absolute-physical-node-executable> <retained-node-SHA256>
<absolute-pinned-soljson-file> <retained-compiler-SHA256>
```

| Command | Additional arguments | Effect |
| --- | --- | --- |
| `review-rh-custody-deployment` | None | Compile and review; no output file. |
| `prepare-rh-custody-deployment` | New absolute private packet path | Compile, construct, write exclusively, fsync and re-read. |
| `check-rh-custody-deployment` | Retained absolute packet path and independently retained SHA256 | Recompile all original inputs and compare the packet byte for byte. |

On Linux/macOS, the request and packet use current-owner mode-0600 regular
single-link files in existing current-owner mode-0700 leaf directories. No
unverified filesystem/ACL fallback exists on other platforms. Source is public
but must be an absolute pinned regular file. Existing output is never overwritten;
partial files remain for separate review after failure. Rechecking a modified
packet with its own new checksum cannot make it agree with the original inputs.
The private packet contains creator, predicted custody, unsigned wire bytes and
input/toolchain pins. Never publish populated requests or packets. JSON/string
reports expose only verification flags, not addresses, paths or wire bytes.

## Signed creation and conditional execution review

`check-rh-custody-deployment-transaction` uses the same eight original arguments
followed by an absolute private signed binary-envelope file and its independently
retained SHA256. It recompiles original materials, recovers the exact sender and
checks chain, nonce, fees, zero value, no destination/access list/authorizations
and identical constructor bytes. Reports contain no addresses or signed bytes.
This command verifies supplied signing output; it does not sign or broadcast.

The compiler worker also returns only the two reviewed immutable declaration
IDs, identified from its actual AST. Token and asset-binding words are derived
from the fixed constructor semantics and predicted CREATE address. There is no
caller-selected replacement map or arbitrary immutable wildcard. Actual pinned
God EVM creation must reproduce the resulting runtime exactly in acceptance.

`CheckCustodyDeploymentExecution` combines the signed creation with complete
transaction/receipt trie roots and a contract account/code proof at the **same
supplied header**. It derives creation identity rather than trusting JSON
`contractAddress` metadata. Successful execution requires the exact compiled
runtime, including both known immutable values; a reverted receipt cannot claim
created code. Cumulative Gas, selected transaction, receipt type and supplied
execution-fee arithmetic are checked. The header's origin, RH settlement,
additional source fees and mutable constructor storage are not authenticated.
These conditional checks never enable financial custody or native credits.

## Durable simulation creation recovery

The explicit `CustodyCreationSimulationJournal` composes those checks with a
private append-only three-stage history:

```text
prepared -> dispatch-unknown -> conditional-observation
```

Open with all original pinned source/request/compiler/signed-file inputs. The
Linux/macOS disk adapter owns an existing empty private directory exclusively;
reopen never creates a missing lease. Records are new mode-0600, single-link,
fsynced files; duplicate, partial, missing, shared, linked or changed history is
refused without repair. A failed constructor leaves storage ownership with the
caller, who must close it; any complete or partial record must be retained for
review rather than reinitializing the directory.

Before handoff, independently retain `ExpectedHandoffPin`. Pass that exact pin
to `TakeForSimulation`; durable UNKNOWN is written and read back **before** one
detached signed-byte export. The prepared pin cannot reopen a newer history.
After interruption, the pre-retained expected UNKNOWN pin allows exact reopen
without deriving trust from the current unreviewed file. UNKNOWN or observed
history cannot export a second submission. `Observe` rechecks complete original
inputs and retains one exact conditional receipt/code identity; an identical
observation is idempotent, a conflicting one is refused. All history prefixes
are rechecked, not just the latest file. Ambiguous writes freeze that handle.

This journal supplies no signer or broadcaster. It is **not a shared source
account nonce manager** across creation and existing method-call journals,
hardware key custody, independent finality or protection from an operator
rolling back all history and retained external pins. Conditional observation
does not release nonce/Gas reservations, and dispatch remains UNKNOWN as a
financial statement even after a matching local observation. Production use
still needs coordinated source-account ownership and authenticated receipt/
state/finality handling. No production journal or submission CLI is installed.

## Financial boundary

Preparation and retained-file verification leave signing, broadcasting,
deployment, token-state trust and real-asset readiness false. There is no
deployment retry or production signing CLI. This closes a source-creation
preparation gap; it does not reserve an account nonce or connect creation to the
existing durable method-call signing journal. A real deployment needs separate
approval of the prototype's limits/control policy, signer custody, exact source
network, live account/nonce, complete fees, bounded production submission,
authenticated receipt/state/finality reconciliation and safe failure recovery.
Only then can confirmed custody state bind the native bridge. The initial
founder-operated topology remains one host, not independent distributed custody.

See [bridge trust and accounting](BRIDGE.md), [launch review](LAUNCH_REVIEW.md)
and [verification limits](VERIFICATION.md). No prototype balances are promoted
into authenticated RH backing by this tool.
