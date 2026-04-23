# Prompt for Codex - Declarative Multi-Blockchain Orchestrator in Go

> Historical document for the project's initial direction.
> For behavior implemented in the current codebase, use `README.md` and `docs/`.

You are a **principal platform engineer** focused on **distributed systems**, **developer tooling**, **infrastructure as code**, **orchestration**, **containers**, and **blockchain operations**.

Your mission is to **design and start implementing** an open-source project called **BGorch - The Blockchain Gorchestrator**, a **declarative multi-blockchain orchestrator** for **deploy**, **configuration**, **bootstrap**, **upgrade**, **backup/restore**, **observability**, and **lifecycle management** of blockchain nodes and clusters.

## Product context

I want a product that works for **any blockchain**, not just Cosmos, Ethereum, Bitcoin, and so on.

That means the design **cannot couple the core** to a single chain family. Instead, the system must have:

1. a **generic orchestration core**;
2. a **common layer** for cross-chain concepts;
3. **plugins/adapters per blockchain family**;
4. **profiles per specific chain**;
5. **execution backends** independent from the chain.

## Main language directive

- **Prefer Go as the primary language**.
- Only choose another language for a component if there is a **strong, explicit technical justification**.
- Even if a subcomponent uses another language, the **main core/control plane/CLI** must remain in **Go**.
- Document every exception in an ADR.

## What I do NOT want

- I do not want to start as "just a Terraform provider".
- I do not want to start as "just an Ansible collection".
- I do not want a project locked to a specific chain.
- I do not want a shallow Docker Compose wrapper.
- I do not want a project that only does `docker run` with loose templates.
- I do not want a design that assumes every chain is a single process.
- I do not want a design that assumes Kubernetes is the only runtime.
- I do not want a design where chain-specific details leak into the core.

## What I do want

I want a **first-party declarative core** in Go, with **desired state**, **plan/apply**, **idempotent reconciliation**, **drift detection**, **deterministic configuration rendering**, and **adapters/backends** for different environments.

Treat:

- **Terraform** as a possible **infrastructure/provisioning adapter**;
- **Ansible** as a possible **host bootstrap/configuration adapter**;
- **Dockerfile** as a packaging mechanism;
- **Docker Compose** as a local/simple backend/orchestrator;
- **Kubernetes** as an advanced backend for stateful workloads;
- **SSH + systemd** as an important backend for bare metal/VMs;
- and leave room for the product to grow into other targets later.

## Architecture strategy

Design the system with these layers:

### 1. Declarative core

Responsible for:

- reading YAML/JSON specs;
- validating schema and rules;
- computing plans;
- reconciling desired state vs current state;
- applying changes idempotently;
- generating outputs, status, events, and diagnostics.

### 2. Common domain model

Generic concepts shared by many chains:

- cluster
- node
- node set / node pool
- role
- workload
- process set
- data directory
- config files
- storage
- secrets
- sync/bootstrap strategy
- snapshot/backup policy
- restart policy
- health checks
- upgrade strategy
- observability
- networking
- resource sizing
- lifecycle hooks

### 3. Blockchain-family extensions

Each family must be able to:

- validate specific fields;
- render configuration files;
- declare required processes;
- define sync/bootstrap strategies;
- define health checks;
- declare compatibility and limits;
- expose extensions without polluting the core.

### 4. Execution backends

Start with a clear interface that can support:

- Docker Engine
- Docker Compose
- SSH + systemd
- Kubernetes
- Ansible adapter
- Terraform adapter

### 5. State and reconciliation

The system must have a clear mechanism for:

- desired state
- observed state
- diff
- plan
- apply
- verify
- rollback/repair
- locks and protection against dangerous concurrent operations

## Mandatory first step: research

Before implementing the core, perform **guided research using official documentation** and produce a document at:

`docs/research/infra-foundations.md`

That document must summarize **best practices and design implications** for the project based on:

1. Dockerfile / Docker Build
2. Docker Compose
3. Ansible
4. Terraform
5. Kubernetes controllers/operators
6. Kubernetes for stateful workloads
7. Go modules / Go project organization

### Research rules

- Prioritize **official documentation**.
- Include official links in the document.
- For each technology, extract:
  - the mental model;
  - the proper responsibilities of that technology;
  - relevant best practices;
  - what **should not** be pushed into it;
  - how that affects the design of `bgorch`.

### Expected research output

At the end of that document, create a section:

`Design Implications for bgorch`

With a table in this format:

| Technology | Best practice | Risk if used incorrectly | Decision in bgorch |
|------------|---------------|--------------------------|--------------------|

## Expected project outcome

I want a project foundation that can evolve toward these goals:

- support **multiple blockchain families**;
- support **multiple processes per node**;
- support **persistent state**;
- support **bootstrap by snapshot / state sync / restore / genesis / custom**;
- support **archive, pruned, validator, RPC, sentry, full node, relayer, observer** roles;
- support **local deploy, VMs, and Kubernetes**;
- support **plan/apply/status/doctor/render**;
- remain extensible without rewriting the core.

## Desired abstraction model

You must design the system in **two schema layers**:

### Layer 1: common and portable schema

Include only concepts that make sense cross-chain.

### Layer 2: specific extensions

Each family plugin can declare:

- additional fields;
- validations;
- rendering;
- helper processes;
- bootstrap logic;
- upgrade rules;
- family-specific observability.

### Important rule

Do not place chain-specific fields in the common schema without a strong justification.

When something is too specific, use:

- `familyConfig`
- `pluginConfig`
- or an equivalent mechanism

...as long as there is **typed validation** and not just an uncontrolled `map[string]any`.

## Design principles

1. **Go-first**
   - CLI, engine, planner, reconciler, and plugins should preferably be in Go.

2. **Idempotency**
   - Repeating `apply` must not break the environment.
   - The system must converge toward desired state.

3. **Determinism**
   - Same spec + same context => same artifacts/renders.

4. **Security**
   - Do not bake secrets into images.
   - Do not leak keys in logs.
   - Treat validator keys / wallet keys / signing keys with extreme care.
   - Allow future integration with secret stores.

5. **Drift detection**
   - Detect divergence between desired and current state.

6. **Extensibility**
   - Adding a new chain family should be localized work.

7. **Multiple runtimes**
   - Support both containers and host binaries.
   - Not every chain should require containers.

8. **Stateful by design**
   - Do not treat a stateful node like a stateless app.

9. **Plan first**
   - Every important change must go through `plan`.

10. **Built-in observability**
    - Logs, metrics, status, and diagnostics from the beginning.

11. **Pragmatism**
    - Small MVP, solid architecture.
    - Do not try to support every chain in the first commit.

## Scenarios the design must support

The system must be able to model:

### Case A - Simple single-process chain

A single daemon with:

- binary or image
- ports
- directories
- config file
- persistent volume
- restart policy
- backup policy

### Case B - Multi-process chain

Generic example:

- execution client
- consensus client
- validator
- sidecar/exporter/proxy

The design must support one "logical node" composed of **multiple processes/workloads**.

### Case C - Host deploy

Without containers. Using:

- binary download
- directory creation
- templates
- systemd
- health checks

### Case D - Container deploy

Using:

- Dockerfile
- Docker Engine
- Docker Compose
- named volumes / bind mounts

### Case E - Kubernetes deploy

Using the right resources for stateful workloads, with PVCs, config, and secrets.

### Case F - Unknown custom blockchain

The user provides:

- image or binary;
- command/args;
- templates;
- ports;
- volumes;
- probes;
- hooks;
- strategy plugins;

and the system still works without requiring hardcoded support.

## MVP direction

For the MVP, implement **the generic engine first** and only then **reference integrations**, not the other way around.

### Mandatory MVP

1. **Go CLI**
2. **Versioned schema**
3. **Validation**
4. **Render**
5. **Plan**
6. **Apply**
7. **Status**
8. **Doctor**
9. **Docker Compose backend**
10. **SSH + systemd backend**
11. **Generic `generic-process` plugin**
12. **At least 1 real reference plugin**
13. **Unit tests and golden tests**
14. **Documentation and examples**

### Suggested real reference plugin

Choose one path:

- `bitcoin-core`
- `ethereum-stack`
- `cometbft-family`
- another justified option

But **do not** make the design depend on it.

## High-level CLI interface

Design something like:

```bash
bgorch init
bgorch validate -f examples/generic-node.yaml
bgorch render -f examples/generic-node.yaml
bgorch plan -f examples/generic-node.yaml
bgorch apply -f examples/generic-node.yaml
bgorch status -f examples/generic-node.yaml
bgorch doctor -f examples/generic-node.yaml
bgorch backup -f examples/generic-node.yaml
bgorch restore -f examples/generic-node.yaml
```

## Suggested resource model

You may adjust names, but I want something in this direction:

- `ChainCluster`
- `NodePool`
- `Node`
- `WorkloadSet`
- `StoragePolicy`
- `SyncPolicy`
- `UpgradePolicy`
- `BackupPolicy`
- `ObservabilityPolicy`
- `SecretRef`
- `ChainFamily`
- `ChainProfile`
- `RuntimeBackend`

Or an equivalent model, as long as it is well justified.

## Suggested plugin interface

Design a Go interface equivalent to something like:

```go
type ChainPlugin interface {
    Name() string
    Family() string
    Capabilities() Capabilities

    Validate(spec *ChainClusterSpec) []Diagnostic
    Normalize(spec *ChainClusterSpec) error

    RenderConfig(ctx context.Context, req RenderRequest) ([]RenderedArtifact, error)
    BuildWorkloads(ctx context.Context, req BuildRequest) ([]WorkloadSpec, error)

    HealthChecks(spec *ChainClusterSpec) []HealthCheck
    BootstrapPlan(spec *ChainClusterSpec) ([]Action, error)
    BackupPlan(spec *ChainClusterSpec) ([]Action, error)
    RestorePlan(spec *ChainClusterSpec) ([]Action, error)
    UpgradePlan(spec *ChainClusterSpec, from, to Version) ([]Action, error)
}
```

You may change the interface if you find a better design, but preserve the core idea:

- validation;
- rendering;
- workload description;
- lifecycle hooks;
- operational strategies.

## Suggested backend interface

Design a Go interface equivalent to:

```go
type Backend interface {
    Name() string

    ValidateTarget(ctx context.Context, target Target) []Diagnostic
    Observe(ctx context.Context, scope Scope) (ObservedState, error)

    Plan(ctx context.Context, desired DesiredState, observed ObservedState) (ExecutionPlan, error)
    Apply(ctx context.Context, plan ExecutionPlan) (ApplyResult, error)
    Verify(ctx context.Context, desired DesiredState) (VerificationResult, error)
}
```

## Desired internal architecture

Structure the project roughly like:

```text
bgorch/
  cmd/
    bgorch/
  internal/
    app/
    cli/
    domain/
    api/
    planner/
    reconcile/
    renderer/
    backend/
      compose/
      sshsystemd/
      kubernetes/
      ansible/
      terraform/
    chain/
      generic/
      bitcoin/
      ethereum/
      cometbft/
    state/
    secrets/
    observability/
    doctor/
  examples/
  docs/
    research/
    adr/
  test/
```

Do not follow this blindly; refine it if you find a better structure, but keep clear responsibility boundaries.

## Important implementation requirements

### 1. Versioned API

- Use something like `v1alpha1`.
- Prepare the ground for future evolution.

### 2. Deterministic rendering

- Templates and generated files must be testable.
- Use golden tests for config rendering.

### 3. Local project state

- Start simple.
- A local state directory plus locks is acceptable.
- Abstract the layer so it can later be replaced by SQLite/Postgres/etcd if needed.

### 4. Safe operations

- `plan` before `apply`;
- `--dry-run`;
- post-apply verification;
- clear messages;
- careful handling of partial failures.

### 5. Logs and diagnostics

- structured logging;
- actionable errors;
- human-readable output and, if possible, JSON.

### 6. Tests

Include:

- unit tests;
- golden tests;
- planner tests;
- render tests;
- minimal integration tests for the Compose backend;
- spec validation tests.

### 7. Examples

Deliver complete examples for:

- generic single-process node
- generic multi-process node
- deploy via Docker Compose
- deploy via SSH + systemd
- one real reference plugin

## Best practices I want embedded in the design

Without limiting the work to these points, I want the research and implementation to consider:

- small, secure images;
- reproducible builds;
- clear separation between build and runtime;
- persistent volumes for stateful data;
- never storing secrets inside images;
- idempotency;
- composition and reuse;
- validation before apply;
- separation of concerns between infra, bootstrap, and operations;
- rollback/repair support;
- health checks;
- readiness / liveness / startup semantics when applicable;
- versionable generated artifacts;
- predictable behavior on re-execution.

## What the design must NOT assume

- that every node uses Docker;
- that every node uses Kubernetes;
- that every chain uses a single database/local state;
- that every chain supports pruning the same way;
- that every chain has the same flags;
- that every upgrade is rolling;
- that every bootstrap happens through snapshots;
- that Terraform should understand the chain's internal details;
- that Ansible should become the product core.

## Correct approach for "any blockchain"

Model the problem like this:

### Common layer

Portable fields:

- name
- family
- profile
- runtime/backend
- workloads
- volumes
- networks
- ports
- resources
- rendered files
- env
- command/args
- restart policy
- sync/bootstrap category
- backup policy
- observability
- secret refs

### Specific layer

Plugin-owned fields:

- specific pruning knobs
- specific DB backend
- specific flags
- specific TOML/YAML/JSON config
- specific sidecars
- specific topologies
- specific bootstrap rules
- specific upgrade rules

## Backends that must exist in the design

### Backend 1 - Docker Compose

It must:

- generate a Compose file;
- model services, networks, and volumes;
- support named volumes/bind mounts;
- support health checks/restart policies;
- bring up and tear down local environments.

### Backend 2 - SSH + systemd

It must:

- prepare directories;
- copy/render configs;
- install or place binaries;
- generate systemd units;
- start/stop/restart services;
- observe status.

### Backend 3 - Kubernetes

Even if the MVP does not implement everything, leave the architecture ready for:

- StatefulSets
- PVCs
- ConfigMaps / Secrets
- Services
- probes
- controlled rolling updates
- affinity / anti-affinity / tolerations where appropriate

### Backend 4 - Terraform adapter

The Terraform adapter must focus on:

- infrastructure provisioning;
- VMs;
- disks;
- networks;
- buckets;
- security groups/firewall;
- Kubernetes clusters.

Do not push into Terraform what belongs to the runtime reconciler.

### Backend 5 - Ansible adapter

The Ansible adapter must focus on:

- host bootstrap;
- templates;
- directory configuration;
- file distribution;
- systemd;
- handlers/restarts;
- inventory integration.

Do not turn Ansible into the main control plane.

## Security and keys

Treat these as first-class requirements:

- private keys;
- validator keys;
- wallet keys;
- mnemonics;
- tokens;
- RPC credentials;
- cloud credentials.

I want:

- a `SecretRef` abstraction;
- support for env/file/external provider;
- never logging secrets;
- never serializing secrets unnecessarily;
- a future path toward KMS/Vault/SOPS or equivalents.

## Observability

Include from the beginning:

- structured logs;
- health model;
- status model;
- events/diagnostics;
- room for metrics;
- future integration with Prometheus/Grafana/Loki or equivalents.

## Expected Codex output

I want you to work in phases and **not try to code everything at once without thinking**.

### Phase 0 - Research and architecture

Deliver:

1. executive summary of the problem;
2. `docs/research/infra-foundations.md`;
3. `docs/adr/0001-core-architecture.md`;
4. `docs/adr/0002-plugin-model.md`;
5. `docs/adr/0003-backend-model.md`;
6. proposed `v1alpha1` schema;
7. initial repository tree.

### Phase 1 - Project scaffold

Deliver:

- Go module;
- directory structure;
- minimal CLI;
- spec parser;
- basic validation;
- backend/plugin registries;
- `validate`, `render`, and `plan` commands.

### Phase 2 - Functional MVP

Deliver:

- `generic-process` plugin;
- `docker-compose` backend;
- `ssh+systemd` backend;
- basic planner;
- file rendering;
- examples;
- minimal tests.

### Phase 3 - Real reference

Deliver:

- 1 real reference plugin;
- real examples;
- usage documentation;
- compatibility decisions.

### Phase 4 - Extensibility

Prepare:

- Kubernetes backend;
- Terraform adapter;
- Ansible adapter;
- API/plugin evolution.

## Execution rules

1. **Think before coding**.
2. **Start with research and ADRs**.
3. **Explain trade-offs**.
4. **Prefer structural simplicity in the MVP**.
5. **Avoid overly magical abstractions**.
6. **Avoid unnecessary dependencies**.
7. **Prioritize clear, testable interfaces**.
8. **Do not hide problems; document limits**.
9. **Do not pretend to have universal support without real basis**.
10. **Design for extension, not for initial overengineering**.

## Desired initial delivery in this round

In this first execution, do the following:

1. Produce the **architectural summary**.
2. Produce the **phased implementation plan**.
3. Generate the **initial directory tree**.
4. Create the **initial ADRs**.
5. Create the **initial `v1alpha1` schema**.
6. Build the **Go scaffold**.
7. Implement:
   - `bgorch validate`
   - `bgorch render`
   - `bgorch plan`
   - plugin registry
   - backend registry
   - `generic-process` plugin
   - `docker-compose` backend with Compose file rendering
8. Add **2 complete examples**
9. Add **tests**
10. Clearly document what remains as the next step

## Acceptance criteria

I will consider the delivery good if:

- the core is in Go;
- the design is not locked to a single chain;
- the schema is clean;
- there is clear separation between core, plugins, and backends;
- the Compose backend generates useful artifacts;
- the generic plugin can model an unknown chain;
- the project includes docs, examples, and tests;
- the ADRs explain why the core is not simply Terraform or Ansible;
- the project can evolve toward Kubernetes, Terraform, and Ansible without a rewrite.

## Response format

I want you to respond in this order:

1. **Summary of the proposed architecture**
2. **Main trade-offs**
3. **Implementation phases**
4. **Repository tree**
5. **Files that will be created**
6. **ADR contents**
7. **Initial Go scaffold contents**
8. **Examples**
9. **Tests**
10. **Next steps**

If you need to make assumptions, state them explicitly.

Start now.
