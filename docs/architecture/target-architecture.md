# Target Architecture (User-First)

## Objetivo

Manter o core declarativo/reconciler e reduzir acoplamento da UX, com fronteiras explícitas para evolução de CLI, automação e plugins/backends.

## Estrutura-Alvo

```text
cmd/chainops/
cmd/bgorch/                    # alias legado
internal/cli/
internal/config/
internal/schema/
internal/planner/
internal/renderer/
internal/engine/
internal/state/
internal/output/
internal/doctor/
internal/workspace/
pkg/pluginapi/
plugins/
backends/
profiles/
examples/
docs/
```

## Responsabilidades

- `internal/cli`: command model, help/UX, mensagens de erro, interação.
- `internal/config`: resolução de defaults/config/env/flags e precedência.
- `internal/engine`: fachada estável para pipelines do core.
- `internal/schema`: catálogo de explainability do schema.
- `internal/output`: serialização e renderização uniforme (`table/json/yaml`).
- `internal/workspace`: onboarding e geração de starter specs/profiles.
- `internal/app`: orquestração declarativa de validate/render/plan/apply/status/doctor.
- `internal/planner`: diff determinístico desired vs snapshot.
- `internal/state`: snapshot + lock.
- `pkg/pluginapi`: contrato mínimo versionável para evolução de plugin SDK.

## Regras Arquiteturais

1. CLI não contém lógica de reconciliação.
2. Core não depende de blockchain específica.
3. Planner e executor permanecem separados.
4. Render canônico e plan são side-effect free.
5. Apply concentra mutações e gating de segurança.
6. API de plugin deve ser pequena, tipada e versionável.

## Fluxos

### Fluxo Humano (padrão)

```text
init -> doctor -> render -> plan -> apply -> status
```

### Fluxo CI/CD

```text
validate -> render -o yaml/json -> plan --out -> apply <plan-file> --yes
```

## Decisões-Chave

- `apply` com confirmação por padrão no `chainops`.
- Em não interativo, `--yes` obrigatório para apply destrutivo.
- `diff` existe como visão focada de `plan` (sem noops).
- `render` prioriza config canônica, preservando modo legado de artifacts.

## Evolução Futura

- Extrair plugins/backends built-in para diretórios raiz `plugins/` e `backends/` como módulos externos.
- Expandir `pkg/pluginapi` para ciclo completo (backup/restore/upgrade/logs).
- Adicionar estado observado distribuído (não apenas snapshot local).
