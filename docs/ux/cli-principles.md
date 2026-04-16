# CLI Principles

## 1. Least Surprise

- Comando principal sempre previsível.
- `render` mostra config canônica por padrão.
- `apply` nunca roda silenciosamente em modo não interativo sem `--yes`.

## 2. Progressive Disclosure

- Caminho básico curto: `init -> doctor -> render -> plan -> apply`.
- Features avançadas ficam em flags/subcomandos (`plan --out`, `apply <plan-file>`, `explain`).

## 3. Safe by Default

- `plan`/`render` side-effect free.
- `apply` com confirmação explícita.
- `destroy` é explícito e local-only na implementação atual.

## 4. Explainability

- `explain` para schema/plugins/perfis.
- `doctor` com checks + hints acionáveis.
- Mensagens de erro no formato título/causa/correção/próximo comando.

## 5. Discoverability

- `--help` com exemplos práticos.
- `completion` para shells principais.
- `profile list/show` e `plugin list` para navegação guiada.

## 6. Automation Friendly

- Saída estruturada para comandos principais: `table|json|yaml`.
- Handoff explícito via `plan --out`.
- Config precedence explícita e observável (`context show`).
