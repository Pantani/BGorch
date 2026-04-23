# Infra Foundations Research for BGorch

## Scope

Research guided by official documentation to shape the architecture of `bgorch` across:

1. Dockerfile / Docker Build
2. Docker Compose
3. Ansible
4. Terraform
5. Kubernetes controllers/operators
6. Kubernetes stateful workloads
7. Go modules / Go project organization

## Official Sources

### Docker
- [Dockerfile overview](https://docs.docker.com/build/concepts/dockerfile/)
- [Dockerfile reference](https://docs.docker.com/reference/dockerfile/)
- [Build best practices](https://docs.docker.com/build/building/best-practices/)
- [Compose file reference](https://docs.docker.com/compose/compose-file/)
- [Compose CLI reference](https://docs.docker.com/compose/reference/)
- [Compose application model](https://docs.docker.com/compose/intro/compose-application-model/)

### Ansible
- [Inventory guide](https://docs.ansible.com/ansible/latest/inventory_guide/index.html)
- [Check mode / diff mode](https://docs.ansible.com/projects/ansible/latest/playbook_guide/playbooks_checkmode.html)
- [Reusing playbooks and roles](https://docs.ansible.com/projects/ansible/latest/playbook_guide/playbooks_reuse.html)

### Terraform
- [terraform plan](https://developer.hashicorp.com/terraform/cli/commands/plan)
- [terraform apply](https://developer.hashicorp.com/terraform/cli/commands/apply)
- [Provisioners](https://developer.hashicorp.com/terraform/language/provisioners)
- [Backend configuration](https://developer.hashicorp.com/terraform/language/settings/backends/configuration)
- [S3 backend (locking/versioning guidance)](https://developer.hashicorp.com/terraform/language/backend/s3)
- [Style guide](https://developer.hashicorp.com/terraform/language/syntax/style)
- [Standard module structure](https://developer.hashicorp.com/terraform/language/modules/develop/structure)

### Kubernetes
- [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/)
- [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator)
- [Custom resources](https://kubernetes.io/docs/concepts/api-extension/custom-resources/)
- [StatefulSets](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/)
- [Persistent Volumes](https://kubernetes.io/docs/concepts/storage/persistent-volumes/)
- [Liveness/Readiness/Startup probes](https://kubernetes.io/docs/concepts/configuration/liveness-readiness-startup-probes/)

### Go
- [Managing module source](https://go.dev/doc/modules/managing-source)
- [Managing dependencies](https://go.dev/doc/modules/managing-dependencies)
- [Go Modules reference](https://go.dev/ref/mod)
- [Module release/version workflow](https://go.dev/doc/modules/release-workflow)

---

## 1) Dockerfile / Docker Build

### Mental model

A Dockerfile describes **image build**, not a continuous operations engine and not a cluster-state reconciliation mechanism.

### Proper responsibility

- Package the runtime and process dependencies.
- Produce a small, reproducible image (multi-stage, minimal base, `.dockerignore`, version pinning).
- Define execution defaults (entrypoint/cmd/env/ports).

### Relevant best practices

- Multi-stage builds and clear build/runtime separation.
- Trusted, minimal base images.
- Avoid unnecessary packages.
- Run as a non-root user when possible.
- Deterministic builds with explicit versions.

### What should not be pushed into Dockerfile

- External-environment bootstrap logic.
- Process/node orchestration.
- Secret management (baking secrets into images is an anti-pattern).

### Impact on `bgorch`

- Dockerfile stays a packaging artifact per workload.
- The core does not depend on containers: a workload may be a host binary.
- Secrets must enter through `SecretRef` at runtime, never during build.

---

## 2) Docker Compose

### Mental model

Compose is a declarative model for **local/simple multi-service applications**, with CLI-driven reconciliation (`docker compose up` reevaluates configuration).

### Proper responsibility

- Define services, networks, volumes, health checks, and restart policy.
- Support local/edge environments with relatively simple state.
- Support environment-specific overlays/merges through separate files.

### Relevant best practices

- Canonical, versionable Compose files.
- Named volumes for stateful data.
- Explicit health checks and consistent restart policy.
- Environment overrides in separate files.

### What should not be pushed into Compose

- Infrastructure provisioning (VMs, cloud network, security groups, external storage).
- Advanced multi-cluster rollout control.
- Sophisticated reconciliation policy per chain family.

### Impact on `bgorch`

- Compose is an MVP execution backend.
- `bgorch` generates Compose deterministically from desired state.
- The core stays generic; Compose only consumes already-resolved state.

---

## 3) Ansible

### Mental model

Ansible is procedural/declarative automation for remote host configuration through inventory and playbooks/roles.

### Proper responsibility

- Host bootstrap.
- File/template distribution.
- Directory and systemd service setup.
- Idempotent task execution (with check/diff mode for validation).

### Relevant best practices

- Clear inventory model (static/dynamic).
- Role-based reuse.
- Check mode/diff mode in pipelines.
- Variable and secret control through Vault/indirection.

### What should not be pushed into Ansible

- The product's primary continuous reconciliation control plane.
- A cross-backend global state model.
- Chain-plugin semantics embedded in loose playbooks.

### Impact on `bgorch`

- Ansible enters as a host bootstrap/config adapter.
- Planning and diff stay in the `bgorch` engine.
- `bgorch` decides *what* to apply; Ansible executes *how* on the host.

---

## 4) Terraform

### Mental model

Terraform is IaC for **infrastructure resources** with `plan`/`apply`, state, and backend locking.

### Proper responsibility

- Provision base infrastructure: network, compute, storage, IAM, clusters.
- Manage drift for infrastructure resources through state.
- Validate changes through `plan` before `apply`.

### Relevant best practices

- Two-step plan/apply workflow in automation.
- Remote state with locking and versioning.
- Standard module structure and consistent style.
- `terraform validate` in CI.

### What should not be pushed into Terraform

- Fine-grained runtime operation of blockchain processes.
- Detailed daemon bootstrap and service lifecycle management.
- Provisioners for everything (HashiCorp treats provisioners as a last resort).

### Impact on `bgorch`

- Terraform is a provisioning adapter, not the core.
- The `bgorch` core keeps the operational desired state of nodes/workloads.
- Clear boundary: Terraform delivers the substrate; `bgorch` delivers operations.

---

## 5) Kubernetes controllers/operators

### Mental model

Kubernetes controllers implement a control loop: observe current state and converge toward desired state. An operator extends that pattern with CRDs plus a controller.

### Proper responsibility

- Idempotent declarative reconciliation.
- `spec` (desired) + `status` (observed) model.
- Automation of operational routines for a domain-specific system.

### Relevant best practices

- Separate responsibilities across small controllers.
- Avoid excessive coupling of application data to the API server.
- Use CRDs for operational abstractions, not as the application's database.

### What should not be pushed into controllers/operators

- Arbitrary business-data storage.
- Monolithic logic that mixes every responsibility together.

### Impact on `bgorch`

- The core adopts controller-inspired reconciliation semantics.
- The internal model should cover desired/observed/diff/plan/apply/verify.
- A future Kubernetes backend can map directly to StatefulSets/Services/PVCs.

---

## 6) Kubernetes for stateful workloads

### Mental model

Stateful workloads require stable identity and persistent storage decoupled from pod lifecycle.

### Proper responsibility

- StatefulSet for stable identity and rollout ordering.
- PVC/PV/StorageClass for persistence and disk lifecycle.
- Correct probes (startup/readiness/liveness) with the right semantics.

### Relevant best practices

- Do not treat a stateful node as a stateless deployment.
- Make storage policy explicit (access mode, reclaim policy, class).
- Use startup probes for long initialization.
- Separate readiness from liveness to avoid restart cascades.

### What should not be pushed into generic Kubernetes behavior

- The assumption that one rollout style works for every chain upgrade.
- Ignoring bootstrap/sync requirements and data-lock behavior.

### Impact on `bgorch`

- `StoragePolicy`, `SyncPolicy`, and `UpgradePolicy` belong in the common domain.
- The Kubernetes backend must translate those policies into the right primitives.
- The core health model must preserve startup/readiness/liveness semantics.

---

## 7) Go modules / Go project organization

### Mental model

A Go module is a versionable unit; code is organized into packages with explicit boundaries.

### Proper responsibility

- Keep core/CLI/plugins/backends as cohesive internal packages.
- Manage dependencies through `go.mod`/`go.sum`.
- Evolve APIs through module versioning and semver.

### Relevant best practices

- One primary module at the repository root to reduce operational friction.
- Use `internal/` to encapsulate implementation details.
- Use `cmd/` for entry binaries.
- Run `go mod tidy` and continuous validation in CI.

### What should not be pushed into Go layout

- Mixing stable external APIs with internal details without a boundary.
- Splitting into many modules too early without a real need.

### Impact on `bgorch`

- Use a structure such as `cmd/bgorch`, `internal/*`, `docs/*`, `examples/*`.
- Keep `v1alpha1` in a dedicated package for controlled evolution.
- Use plugin/backend registries as predictable extension points.

---

## Design Implications for BGorch

| Technology | Best practice | Risk if used incorrectly | Decision in BGorch |
|------------|---------------|--------------------------|--------------------|
| Dockerfile/Build | Multi-stage, minimal image, no embedded secrets | Large, insecure images and non-reproducible builds | Treat Dockerfile strictly as packaging; inject secrets through `SecretRef` at runtime |
| Docker Compose | Declarative services/volumes/networks/health checks | Turns into a pseudo control plane without a drift model | Compose backend emits deterministic artifacts from the core |
| Ansible | Idempotent roles plus check/diff mode | Becomes the main engine and absorbs chain rules | Optional bootstrap/host-configuration adapter |
| Terraform | Plan/apply with remote state locking | Pushes chain runtime behavior into provisioners | Infrastructure adapter (VM/network/storage), not runtime orchestrator |
| Kubernetes controllers/operators | Control loop plus spec/status separation | Monolithic controller and improper coupling | `bgorch` core follows idempotent reconciliation by design |
| Kubernetes stateful | StatefulSet + PVC + correct probes | Treats a stateful node as a stateless app | Model `StoragePolicy`, `SyncPolicy`, and `UpgradePolicy` in the common domain |
| Go modules/layout | `cmd` + `internal` + one initial module | Weak boundaries and brittle API evolution | Go-first core/CLI, versioned API, registry-based extensions |
