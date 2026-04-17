# `internal/doctor`

Model for health-check style command outputs (`chainops doctor`).

## Responsibility

- define `Report` aggregate and `Check` entries;
- standardize status taxonomy (`pass`, `warn`, `fail`);
- provide helpers used by app pipeline to build actionable doctor output.

## Entrypoints

- `NewReport()`
- `(*Report).Add(...)`
- `Report.HasFailures()`
- `Report.HasWarnings()`

## Design Notes

- report model is transport-focused (serializable to table/json/yaml);
- check semantics are produced by `internal/app` doctor pipeline, not by this package;
- timestamps are UTC for deterministic cross-environment interpretation.
