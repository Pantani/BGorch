# CLI Redesign Migration

## Contexto

A CLI evoluiu de `bgorch` (MVP técnico) para `chainops` (UX user-first).

Objetivo:

- reduzir carga cognitiva
- aumentar segurança operacional
- melhorar automação e explainability

## Compatibilidade

- `cmd/bgorch` continua disponível como alias legado.
- Fluxos antigos continuam funcionando com compatibilidade de flags críticas (`-o` como output-dir em comandos legados).

## Mudanças Principais

1. Novo comando raiz: `chainops`
2. Novo fluxo recomendado: `init -> doctor -> render -> plan -> apply`
3. `render` agora prioriza configuração canônica resolvida
4. `plan --out` e `apply <plan-file>`
5. `apply` com gating de segurança (`--yes` em não interativo)
6. `explain`, `diff`, `plugin/profile/context`, `completion`

## Mapeamento de Comandos

| Legado (`bgorch`) | Novo (`chainops`) | Observação |
|-------------------|-------------------|------------|
| `validate` | `validate` | Mantido |
| `render -o <dir>` | `render --write-artifacts --artifacts-dir <dir>` | `-o <dir>` ainda aceito por compat |
| `plan` | `plan` | `--out` novo |
| `apply -f spec` | `apply -f spec --yes` | `--yes` exigido no não interativo |
| `status` | `status` | `--output` table/json/yaml |
| `doctor` | `doctor` | checks acionáveis |
| `tui/ui` | `bgorch tui/ui` | legado |

## Recomendações de Pipeline

Antes:

```bash
bgorch plan -f cluster.yaml
bgorch apply -f cluster.yaml
```

Depois:

```bash
chainops plan -f chainops.yaml --out plan.json --output json
chainops apply plan.json --yes --output json
```

## Deprecações Planejadas

- `bgorch` será mantido durante fase de transição.
- Nova funcionalidade será adicionada prioritariamente no namespace `chainops`.
