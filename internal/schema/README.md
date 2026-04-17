# `internal/schema`

Explainability catalog used by `chainops explain`.

## Responsibility

- maintain curated docs for schema paths,
- normalize user query paths,
- return structured field docs and examples.

## Entrypoints

- `Lookup(path string)`
- `Doc` / `FieldDoc` payloads

## Caveats

- this is curated help content, not runtime validation logic,
- validation source of truth remains `internal/validate` + plugin/backend validators.
