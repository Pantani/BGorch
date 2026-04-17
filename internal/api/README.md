# `internal/api`

Versioned internal schema packages consumed by spec loader, validator, plugins, and backends.

## Responsibility

- group API versions under a stable directory boundary (`v1alpha1`, future versions);
- keep schema type evolution explicit by package/version;
- provide a clear bridge between YAML schema and Go model types.

## Current Version

- `v1alpha1`: `ChainCluster` model (`bgorch.io/v1alpha1`).

## Evolution Rule

Breaking schema changes should introduce a new version package (for example `v1beta1`) instead of mutating existing version contracts in place.
