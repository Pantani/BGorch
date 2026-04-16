# Configuration Model

## 1. Spec de Desired State (`ChainCluster`)

Arquivo principal de operação (ex.: `chainops.yaml`) com schema `bgorch.io/v1alpha1`.

Esse arquivo define:

- família e plugin (`spec.family`, `spec.plugin`)
- backend/runtime (`spec.runtime`)
- topologia de nós/workloads (`spec.nodePools`)
- políticas (`backup`, `upgrade`, `observe`)
- extensões tipadas (`pluginConfig`, `backendConfig`)

## 2. CLI Runtime Config

Configuração do comportamento da CLI (não do cluster), via:

- `--config` (arquivo)
- `CHAINOPS_*` (env)
- flags

Exemplos: `state-dir`, `artifacts-dir`, `output`, `yes`, `non-interactive`.

## 3. Render Canônico

`chainops render -f chainops.yaml -o yaml` mostra a configuração final resolvida com defaults aplicados.

Garantias:

- side-effect free por padrão
- determinístico para mesmo input
- útil para review/auditoria antes de `plan`/`apply`

## 4. Plan File (Handoff)

`chainops plan --out plan.json` gera arquivo de plano versionado.

`chainops apply plan.json --yes` aplica usando `sourceSpec` do plano.

Uso recomendado:

- revisão humana do plano
- aprovação em pipeline
- apply posterior com trilha de auditoria

## 5. Padrões de Segurança

- `apply` exige confirmação no modo interativo.
- Em não interativo, requer `--yes`.
- `plan` e `render` devem permanecer side-effect free.
