# Error Style Guide

## Objetivo

Mensagens devem responder, nessa ordem:

1. O que falhou
2. Causa provável
3. Como corrigir
4. Próximo comando recomendado

## Template

```text
<Title claro>
Cause: <causa provável>
Hint: <como corrigir>
Next: <comando recomendado>
```

## Regras

- Evitar mensagens genéricas (`invalid config`, `failed`).
- Incluir campo/path quando aplicável.
- Priorizar ação concreta, não teoria.
- Não esconder impacto de flags conflitantes.

## Exemplos

### Ruim

```text
invalid config
```

### Bom

```text
Invalid CLI configuration.
Cause: read config file "chainops-cli.yaml": no such file or directory
Hint: Check --config path, environment variables, and flag values.
Next: chainops context show
```

### Ruim

```text
unsupported output
```

### Bom

```text
Unsupported output format.
Cause: "xml" is not supported.
Hint: Use table, json, or yaml.
Next: chainops plan -f chainops.yaml --output json
```

## Situações críticas

- Operações destrutivas devem mencionar explicitamente confirmação/`--yes`.
- Erros de preflight devem sempre sugerir `doctor` quando aplicável.

## Convenções no código

- Use `internal/output.ActionableError(...)` para erros de UX.
- Preserve mensagens técnicas completas em logs/stack internos quando necessário.
