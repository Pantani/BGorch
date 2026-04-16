# Chainops

Chainops é um orquestrador declarativo, Go-first, para operações multi-blockchain.

Foco do produto:

- desired state + plan/apply/reconcile
- UX previsível para operadores
- extensibilidade por plugin/família/backend
- compatibilidade com múltiplos runtimes (compose, ssh+systemd, kubernetes, terraform, ansible)

`bgorch` continua disponível como alias legado para migração incremental.

## Para Quem

- Desenvolvedor local: subir stack rápido, validar, destruir sem atrito.
- Operador/SRE: diff claro, preflight, apply seguro, saída estável para automação.
- Operador blockchain: escolher família/profile/backend sem precisar conhecer internals.
- Autor de plugin: evoluir suporte de famílias sem acoplar no core.

## Mental Model

1. Você declara estado desejado (`ChainCluster`).
2. `render` mostra configuração canônica resolvida.
3. `plan` calcula diff determinístico sem efeitos colaterais.
4. `apply` reconcilia estado com confirmação explícita.
5. `status`/`doctor` explicam convergência e problemas.

## Quickstart (One Obvious Path)

```bash
# 1) Bootstrap do workspace
chainops init --profile local-dev --name demo

# 2) Preflight
chainops doctor -f chainops.yaml

# 3) Configuração canônica resolvida
chainops render -f chainops.yaml -o yaml

# 4) Preview de mudanças
chainops plan -f chainops.yaml --out plan.json

# 5) Reconciliação segura
chainops apply plan.json --yes
```

## Fluxo Principal

```bash
chainops init
chainops doctor
chainops render
chainops plan
chainops apply
```

## Comandos Principais

- `init`: cria projeto/spec inicial (interativo ou não interativo).
- `explain`: explica schema/campos/plugins/perfis.
- `render`: exibe config canônica resolvida; também suporta render de artifacts.
- `plan`: preview side-effect free; suporta `--out`.
- `apply`: reconcilia desired state; suporta `apply <plan-file>`.
- `status`: desired vs observed.
- `logs`: detalhes de observação de runtime.
- `doctor`: checks acionáveis de preflight/convergência.
- `destroy`: teardown local explícito (artifacts + snapshot).
- `backup`/`restore`/`upgrade`: inspeção de policy (adapters runtime pendentes).
- `plugin`, `profile`, `context`, `completion`, `version`.

Referência detalhada: [docs/reference/cli.md](docs/reference/cli.md).

## Exemplos

- `examples/starter-local-dev.yaml`
- `examples/starter-vm-single.yaml`
- `examples/generic-single-compose.yaml`
- `examples/generic-single-ssh-systemd.yaml`
- `examples/generic-single-kubernetes.yaml`
- `examples/generic-infra-terraform.yaml`
- `examples/generic-bootstrap-ansible.yaml`
- `examples/cometbft-single-validator.yaml`

## Configuração e Precedência

Precedência explícita:

```text
defaults < config file < env vars < flags
```

Documentação:

- [docs/reference/config-precedence.md](docs/reference/config-precedence.md)
- [docs/reference/configuration-model.md](docs/reference/configuration-model.md)

## Arquitetura (Resumo)

Camadas principais:

- `internal/cli`: UX/command model (Cobra).
- `internal/config`: resolução de config (Viper, precedência explícita).
- `internal/engine`: façade do core declarativo.
- `internal/schema`: explain docs.
- `internal/workspace`: onboarding/profiles/templates.
- `internal/app`, `planner`, `state`, `backend`, `chain`: core reconcile.
- `pkg/pluginapi`: API versionável para evolução de SDK.

Detalhes: [docs/architecture/target-architecture.md](docs/architecture/target-architecture.md).

## Qualidade

```bash
go test ./...
make verify
```

Cobertura adicionada para:

- precedência de config
- render canônico side-effect free
- plan side-effect free
- help output golden
- fluxo chainops (`init/plan/apply` com `--out`/`--yes`)

## Migração

Guia de migração CLI: [docs/migrations/cli-redesign.md](docs/migrations/cli-redesign.md).

## Documentação UX

- [docs/ux/cli-principles.md](docs/ux/cli-principles.md)
- [docs/ux/personas.md](docs/ux/personas.md)
- [docs/ux/user-journeys.md](docs/ux/user-journeys.md)
- [docs/ux/error-style-guide.md](docs/ux/error-style-guide.md)
