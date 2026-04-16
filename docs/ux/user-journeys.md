# User Journeys

## Journey A: Primeiro Sucesso (Local Dev)

```bash
chainops init --profile local-dev --name demo
chainops doctor -f chainops.yaml
chainops render -f chainops.yaml -o yaml
chainops plan -f chainops.yaml
chainops apply -f chainops.yaml --yes
```

Resultado esperado:

- config válida
- plano compreensível
- apply seguro

## Journey B: Operação com Handoff de Plano

```bash
chainops plan -f chainops.yaml --out plan.json --output json
# revisão/aprovação externa
chainops apply plan.json --yes --output json
```

Resultado esperado:

- trilha de auditoria
- execução previsível em CI

## Journey C: Troubleshooting

```bash
chainops status -f chainops.yaml --observe-runtime
chainops logs -f chainops.yaml
chainops doctor -f chainops.yaml --observe-runtime
```

Resultado esperado:

- causa provável clara
- hints acionáveis

## Journey D: Descoberta de Schema e Perfis

```bash
chainops explain ChainCluster
chainops explain ChainCluster.spec.runtime
chainops explain plugin generic-process
chainops profile list
chainops profile show vm-single
```

Resultado esperado:

- aprendizado incremental
- menos tentativa e erro
