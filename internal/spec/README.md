# `internal/spec`

Spec package handles YAML loading, default application, and node expansion.

## Responsibility

- decode `ChainCluster` from YAML (`LoadFromFile`),
- apply implementation defaults (`ApplyDefaults`),
- expand node pools into concrete node instances (`ExpandNodes`).

## Expansion Semantics

- replica expansion derives stable node names,
- pool role can backfill node role when template role is empty,
- output is consumed by plugins/backends for deterministic artifact/service generation.

## Important Constraint

`ApplyDefaults` mutates the loaded object in-memory before validation/build.
All command pipelines assume defaults are applied before plugin/backend processing.
