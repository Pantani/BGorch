# `internal/output`

Output and error-formatting utilities shared by CLI flows.

## Responsibility

- normalize output format names (`table/json/yaml`),
- encode payloads for machine-readable formats,
- render tabular output for operator UX,
- produce actionable error messages with cause/hint/next-command structure.

## Entrypoints

- `NormalizeFormat`
- `Encode`
- `WriteTable`
- `ActionableError`
