# `internal/chain`

Plugins encapsulate chain-family behavior without leaking it into core orchestration.

## Contract (`Plugin`)

- `Name`, `Family`, `Capabilities`
- `Validate(spec)`
- `Normalize(spec)`
- `Build(ctx, spec) -> Output`

`Output` is consumed by backends to build final desired state.

## Implementations

- `genericprocess`: generic fallback for arbitrary process-oriented chains.
- `cometbft`: cometbft-oriented defaults/validation and generated config assets.
- `evm`: EVM family plugin with typed client/network config.
- `solana`: Solana family plugin with typed validator/RPC config.
- `bitcoin`: Bitcoin family plugin with typed node config.
- `cosmos`: Cosmos-SDK family plugin with typed chain config.

## Interaction Rules

- plugin owns family-level validation and normalization,
- plugin may generate family artifacts,
- plugin should not encode runtime-specific orchestration behavior.
