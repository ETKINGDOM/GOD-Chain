# Private token constructor compatibility

The local compatibility APIs reproduce constructor-dependent runtime in
God EVM without changing an issued token, connecting to RH, deploying custody
or enabling assets. They are offline APIs and operator commands, not a public
wallet route or an activation command. The existing no-replacement `recompile-rh-token`
command retains its original behavior.

## Offline operator commands

`rh-token-creation-template <create|create2>` prints an intentionally incomplete
English private replay template. It never invents a creator, creation nonce,
salt, clock, Gas budget or constructor argument. Numeric fields use canonical
decimal strings. `create` requires only `nonce`; `create2` requires only `salt`.
An explicit empty argument list is `0x`, not a missing field. Keep filled inputs
private and retain their complete-file digest separately.

```text
replay-rh-token-creation <absolute-private-config> <config-SHA256> <absolute-private-bundle> <bundle-SHA256> <absolute-node-binary> <node-SHA256> <absolute-known-soljson-file> <compiler-SHA256> <absolute-private-replay> <replay-SHA256> <full-init-SHA256>
```

The three private documents must be owner-held regular files with mode `0600`
in reviewed owner-held leaf directories with mode `0700`. Final symlinks,
hard-link aliases, nonregular files, shared permissions and changed snapshots
are refused. Linux/macOS provide this bounded file workflow; unsupported hosts
fail closed rather than substituting weaker storage checks. The source capsule
limit stays 1 MiB; replay JSON is limited to 40 KiB and its decoded arguments
to 16 KiB. Custody request and parent-proof file limits stay unchanged. This is
not protection from a malicious file owner or an audit of all ancestor paths.

`ReviewPrivateTokenCreation` reads the separately pinned config, source and
replay snapshots, invokes the selected pinned compiler once, replays once and
rereads every original document before returning success. The complete init
digest is also supplied separately; the command does not derive its acceptance
pin from its own output. No source is downloaded, no configured source RPC is
queried, no file is written, and no runtime pin or issued contract is changed.

Success emits only a redacted report with `offlineReplayMatched`, the compile
and exact replay results, and private-snapshot checks. A failed workflow emits
no partial report. Source text, addresses, constructor words, compiler paths
and retained digests are not printed. The three signing/broadcast/real-asset
flags stay false even on success. This command does not certify original
issuance, source state, permissions or finality and cannot sign or submit a
bridge transaction.

## Compile and replay

`CompileTokenCreation` uses the existing explicit, hash-checked local compiler
selection and complete private source capsule. It requests the exact target's
creation code, runtime template, link references and immutable references.
Only existing compiler pins and explicit Cancun settings are supported; linked
libraries remain unsupported. Normal output exposes counts and conditional
flags, not code, source text, compiler paths or operational addresses.

Only that completed compiler path creates the opaque `CompiledTokenCreation`.
Its explicit `Input(arguments)` method exports detached private init bytes for
separate retention/review; it does not build a signed or funded transaction.
`ReproduceTokenCreation` takes this artifact, explicit private replay parameters
and a separately retained SHA-256 of the complete init code including arguments.
It binds the computed CREATE or CREATE2 destination to the source capsule's
configured token before executing anything. CREATE uses the supplied creator
nonce and forbids a salt; CREATE2 uses the supplied salt and forbids a nonce.
No constructor value, creator, height, timestamp or argument is inferred.

One disposable in-memory Cancun VM runs the exact constructor. All returned
runtime bytes must match the retained deployed-runtime declaration. Only the
compiler-declared zero placeholders may change from the runtime template;
every repeated occurrence of one immutable must have the same word. Values
come from actual local constructor execution, not supplied replacement words
or wildcard comparisons. The state database is never exposed or persisted.

## Bounds and supported constructor behavior

| Material or execution limit | Bound |
| --- | --- |
| Complete init code, including arguments | 48 KiB |
| Constructor arguments | 16 KiB |
| Runtime code | 24 KiB |
| Immutable groups / total sites | 64 / 256 |
| Each immutable site | Exactly 32 bytes; disjoint, ordered and in range |
| Execution Gas / instruction steps | 10,000,000 / 200,000 |
| Cooperative replay deadline | Three seconds |

Supported constructors can use their own fresh storage, creator, contract
address and explicit height/time/chain values. External account/code reads,
calls, nested creation, destruction, unprovided origin/block/fee data and
gas-dependent behavior are refused when executed. Opcode bytes inside PUSH
data are not mistaken for instructions. Unsupported behavior is not emulated
with invented source state. Errors, cancellation and panics return a static
refusal and a zero replay report, without partial success.

Exact reproduction is conditional on the supplied compiler and replay inputs.
It does not authenticate original issuance, the RH fork policy, creation
transaction, permissions, source backing or finality. Signing, broadcasting,
RH execution verification and real-asset readiness remain false. Existing
bridge supply and one-for-one transfer rules are unchanged.

## Local acceptance

Use checksum-bound `test-token-creation`, `check-token-creation` and
`fuzz-token-creation`. `test-token-creation-evm` additionally needs explicitly
selected private `GOD_TOKEN_NODE`, `GOD_TOKEN_NODE_SHA256` and
`GOD_TOKEN_SOLJSON_30`, plus retained private custody bytecode. It runs an
actual pinned compiler and a separate reference wrapper/uninstrumented VM;
no mock success response substitutes for compilation.

The private reference asset embeds six constructor values and supplies the
existing token-transfer interface. Tests cover both creation schemes, exact
runtime matching, changed parameters/height/time/address/code, unprovided
environment access, layout bounds, cancellation, concurrency and redaction.
The compiled asset also runs through the existing custody deposit and delayed
return calls, including duplicate and opposite-terminal refusal. These clocks,
balances and keys are synthetic, not actual RH funds or production signers.
`test-token-creation-roundtrip` separately exercises the retained compiled
custody against the real God SDK bank/IAVL ledger and its one-for-one invariants.

`test-token-creation-cli` additionally builds the native operator executable
through the checksum-bound lifecycle workflow and invokes that real executable
with pinned private fixtures. Both creation schemes must reproduce the same
runtime as the separate compiler wrapper/reference VM. Tampered arguments or
pins, changed documents, shared storage and aliases must produce no successful
report. A loopback source endpoint observes zero requests. This target requires
the same explicit private runtime/compiler selections as the actual compiler
tests; it uses the private `tokencreationcli` test tag. Generated executables
and fixture documents are neither installed nor published.

The source publication allowlist must separately review the new production
files before an authorized sanitized upload. Tests, populated source capsules,
replay inputs, addresses, compiler files, keys and binaries remain private.
