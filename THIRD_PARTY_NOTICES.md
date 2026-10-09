# GOD Chain dependency attribution

The isolated bridge tests also use the pinned execution library's in-memory EVM and cryptography. Solidity compilation uses the official [0.8.37 compiler release](https://github.com/argotorg/solidity/releases/tag/v0.8.37) through the local solc-js 0.8.30 wrapper. Its bundled compiler is not used. Compiler provenance and settings are in `UPSTREAM.lock.json`; compiler tools and artifacts are not vendored or redistributed. Their upstream licenses and applicable notices must be reviewed before any future tool or binary redistribution.

God EVM, God SDK and GodCometBFT are project integration names. They do not replace the origin, licenses, package identities or copyright notices of the open-source infrastructure imported by this prototype.

The GodCometBFT lifecycle build helper contains minimal reactor context excerpts from the pinned Apache-2.0 upstream source and applies project modifications for worker admission and joining. The original [Apache license](licenses/GodCometBFT-Apache-2.0.txt) and [NOTICE](licenses/GodCometBFT-NOTICE.txt) are retained verbatim. Generated modified source is explicitly marked; the exact original module copy, full original notices and compiler inputs remain ignored private build material, not a redistributed dependency tree. This patch does not claim original authorship of the upstream reactor or complete future binary license review.

| Component | Upstream technical dependency | Pinned version | License |
| --- | --- | --- | --- |
| God EVM | [cosmos/evm](https://github.com/cosmos/evm/tree/v0.6.3) | v0.6.3 | Apache 2.0 |
| God SDK | [cosmos/cosmos-sdk](https://github.com/cosmos/cosmos-sdk/tree/v0.53.6) | v0.53.6 | Apache 2.0 |
| GodCometBFT | [cometbft/cometbft](https://github.com/cometbft/cometbft/tree/v0.38.21) | v0.38.21 | Apache 2.0 |
| EVM address and signing libraries | [cosmos/go-ethereum](https://github.com/cosmos/go-ethereum/tree/v1.16.2-cosmos-1), through the existing `github.com/ethereum/go-ethereum` replacement | v1.16.2-cosmos-1 | LGPL 3.0 or later for the imported library source |

The upstream EVM project's credits recognize its origins in evmOS and the foundational work of Tharsis, with support from the Interchain Foundation. Those credits remain applicable to imported infrastructure. This project does not claim authorship of that work.

The JSON library dependency `github.com/bytedance/sonic` is pinned to v1.15.4 for compatibility with Go 1.26. The [official release history](https://github.com/bytedance/sonic/releases) records that runtime support and later fixes. This is a documented local compatibility change, not a claim that all transitive dependencies have been audited.

The address encoder and native transaction prototype import the reference's existing execution-library replacement for address checksums, cryptography and EVM-compatible public keys. The library's original copyright and license remain applicable; its `COPYING` and `COPYING.LESSER` files must be reviewed and supplied as applicable before any binary redistribution. The EVM-compatible key package also brings transitive execution and cryptography dependencies. They remain pinned by the existing module graph, and their licenses need separate redistribution review. This prototype does not import or redistribute the upstream node command.

The project's protobuf schema is compiled with the official [Protocol Buffers v33.0](https://github.com/protocolbuffers/protobuf/releases/tag/v33.0) compiler and the reference SDK's `github.com/cosmos/gogoproto` v1.7.2 generator. The tool runs locally and is not vendored or shipped as a node binary. Its verified asset and SHA-256 provenance are recorded in `UPSTREAM.lock.json`; generated Go code retains its generator header. Original imported protobuf annotations, Go packages and applicable notices remain unchanged.

Go downloads the original modules with their license files. No upstream node source, test accounts, wallet fixtures or dependency folders are vendored into this directory. The full dependency graph and checksums are recorded by `go.mod` and `go.sum`. Any future source or binary publication must include the relevant dependency licenses and notices, including those of any additional execution, database and cryptography libraries actually distributed. Imports and version pins alone do not complete binary redistribution review.

## Local browser wallet dependencies

The local disposable browser wallet uses the existing installed `@noble/hashes` 1.8.0 for scrypt and Keccak, `@noble/curves` 2.4.0 for secp256k1 account derivation/signing (with its `@noble/hashes` 2.4.0 dependency), and `@scure/base` 2.4.0 for Bech32 encoding. These upstream libraries retain their MIT licenses and Paul Miller copyright notices. Browser AES-256-GCM uses the platform Web Crypto implementation. Ordinary transfer encoding reuses the unmodified installed `@ethereumjs/rlp` 10.1.3 under MPL-2.0. Its complete license and the covered original `src/index.ts` and `src/errors.ts` source accompany the local build in `licenses.txt`, separately identified from project code. The builder checks exact installed versions and uses esbuild 0.28.1 as a build-only tool. No package was upgraded, protocol dependency changed or dependency tree vendored into production source. Local builds do not authorize public website or binary distribution; a future public release needs scope and license review. Using these libraries does not establish an audit of this wallet integration.

## Test NFT collection dependencies

The disposable NFT collection uses pinned OpenZeppelin Contracts 5.4.0
ERC721Enumerable, ERC721, Base64, Strings and their transitive Solidity
dependencies. The original upstream MIT license and Zeppelin Group Ltd
copyright are retained in [OpenZeppelin.LICENSE](contracts/OpenZeppelin.LICENSE)
and the public browser build's `licenses.txt`. Exact transitive file hashes,
npm archive integrity and the separate solc-js 0.8.30 compiler hash are in
`contracts/NFT.upstream.lock.json`. This compiler is separate from the pinned
bridge compiler; no protocol dependency was upgraded. Imported source retains
its SPDX and copyright headers. The project does not claim original authorship
or an independent audit of these libraries. Local private build material is
not a public dependency tree or binary release.

## Synthetic faucet journal

The separate disposable faucet uses the existing pinned `go.etcd.io/bbolt` v1.4.0-alpha.1 dependency for synchronous transactional journaling. Its upstream MIT license and Ben Johnson copyright remain applicable; the pinned module's complete license accompanies private build material. No database server or new protocol dependency was added or silently upgraded. The faucet also uses the existing God account codec and execution signing libraries with their original dependency licenses. This integration has developer-run tests, not an independent security audit or authorization for public binary redistribution.

## Opt-in address-history index foundation

The separate opt-in history CLI reuses the same pinned `go.etcd.io/bbolt`
v1.4.0-alpha.1 for its synchronous complete-block checkpoint and address index.
The original MIT license and Ben Johnson copyright remain applicable and
accompany the private build material. No dependency version or module checksum
was changed. Reuse of this existing alpha pin is not production-storage
acceptance, an independent audit or authorization to redistribute a binary.
The reviewed public-core scope includes the CLI/module source, not a binary or
the dependency tree. Source publication does not complete redistribution review.

## Restricted native gateway storage and build

The keyless native gateway uses the same pinned `go.etcd.io/bbolt`
v1.4.0-alpha.1 for synchronous reservations and offline inventory copying.
Its MIT license and Ben Johnson copyright remain applicable. This alpha pin
is unchanged and is not accepted production storage or binary redistribution.

The optional gateway build mirrors unchanged `cosmossdk.io/log` v1.6.1 under
its original Apache-2.0 license, retaining all original files and notices in
ignored local build material. Two checksum-bound logging overlays substitute
standard-library JSON for Sonic without altering the original module cache.
`cmd/godbuild` contains project patch instructions, not a vendored logging tree.
The source does not claim upstream authorship or complete future binary license
review. GodCometBFT's original license and notice remain unchanged.
