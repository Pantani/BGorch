# User Journeys

## Journey A: First Success (Local Dev)

```bash
chainops init --profile local-dev --name demo
chainops doctor -f chainops.yaml
chainops render -f chainops.yaml -o yaml
chainops plan -f chainops.yaml
chainops apply -f chainops.yaml --yes
```

Expected result:

- valid config
- understandable plan
- safe apply

## Journey B: Operation with Plan Handoff

```bash
chainops plan -f chainops.yaml --out plan.json --output json
# external review/approval
chainops apply plan.json --yes --output json
```

Expected result:

- audit trail
- predictable CI execution

## Journey C: Troubleshooting

```bash
chainops status -f chainops.yaml --observe-runtime
chainops logs -f chainops.yaml
chainops doctor -f chainops.yaml --observe-runtime
```

Expected result:

- clear probable cause
- actionable hints

## Journey D: Schema and Profile Discovery

```bash
chainops explain ChainCluster
chainops explain ChainCluster.spec.runtime
chainops explain plugin generic-process
chainops profile list
chainops profile show vm-single
```

Expected result:

- incremental learning
- less trial and error
