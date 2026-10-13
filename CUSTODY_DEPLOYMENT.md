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

## Financial boundary

Preparation and retained-file verification leave signing, broadcasting,
deployment, token-state trust and real-asset readiness false. There is no
deployment retry or production signing CLI. This closes a source-creation
preparation gap; it does not reserve an account nonce or connect creation to the
existing durable method-call signing journal. A real deployment needs separate
approval of the prototype's limits/control policy, signer custody, exact source
network, live account/nonce, complete fees, canonical signed-envelope checks,
bounded submission, receipt/finality reconciliation and safe failure recovery.
Only then can confirmed custody state bind the native bridge. The initial
founder-operated topology remains one host, not independent distributed custody.

See [bridge trust and accounting](BRIDGE.md), [launch review](LAUNCH_REVIEW.md)
and [verification limits](VERIFICATION.md). No prototype balances are promoted
into authenticated RH backing by this tool.
