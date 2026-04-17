# `internal/api/v1alpha1`

Schema types for `bgorch.io/v1alpha1` (`ChainCluster`).

## Responsibility

- define portable common model for desired topology,
- define typed backend/plugin extension blocks,
- provide compile-time contract shared by loader/validator/plugins/backends.

## Notes

- defaults are applied in `internal/spec.ApplyDefaults`, not in this package,
- semantic validation lives in `internal/validate` + plugin/backend validators,
- JSON schema reference lives at `docs/schema/v1alpha1.schema.json`.
