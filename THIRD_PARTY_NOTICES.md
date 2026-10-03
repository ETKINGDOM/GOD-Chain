# GOD Chain dependency attribution

God EVM, God SDK and GodCometBFT are project integration names. They do not replace the origin, licenses, package identities or copyright notices of the open-source infrastructure imported by this prototype.

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
