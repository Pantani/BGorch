# `internal/validate`

Core semantic validator for `ChainCluster` independent of plugin/backend internals.

## Responsibility

- enforce required fields and structural constraints,
- validate names, ports, workload modes, mount references and file path safety,
- emit high-signal diagnostics with path + hint.

## Entrypoint

- `Cluster(c *v1alpha1.ChainCluster) []domain.Diagnostic`

## Interaction with plugin/backend validators

Core validator runs alongside plugin/backend validators. A spec can pass shape checks here and still fail plugin/backend-specific constraints.
