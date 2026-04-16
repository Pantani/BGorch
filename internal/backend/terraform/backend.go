package terraform

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Pantani/gorchestrator/internal/api/v1alpha1"
	"github.com/Pantani/gorchestrator/internal/backend"
	"github.com/Pantani/gorchestrator/internal/chain"
	"github.com/Pantani/gorchestrator/internal/domain"
)

const (
	// BackendName is the canonical registry key for the terraform adapter.
	BackendName = "terraform"
	moduleDir   = "terraform"
)

// Backend renders terraform bootstrap artifacts from the generic desired model.
type Backend struct{}

var _ backend.Backend = (*Backend)(nil)

// New returns a terraform backend instance.
func New() *Backend {
	return &Backend{}
}

// Name returns the canonical backend name.
func (b *Backend) Name() string {
	return BackendName
}

// ValidateTarget validates adapter-level constraints and emits actionable diagnostics.
func (b *Backend) ValidateTarget(c *v1alpha1.ChainCluster) []domain.Diagnostic {
	diags := make([]domain.Diagnostic, 0)

	if strings.TrimSpace(c.Spec.Runtime.Backend) != BackendName {
		diags = append(diags, domain.Error(
			"spec.runtime.backend",
			"terraform backend selected with incompatible backend name",
			"use spec.runtime.backend: terraform",
		))
		return diags
	}
	if len(c.Spec.NodePools) == 0 {
		diags = append(diags, domain.Error(
			"spec.nodePools",
			"terraform backend requires at least one nodePool",
			"define nodePools to materialize infra topology variables",
		))
	}
	if strings.TrimSpace(c.Spec.Runtime.Target) == "" {
		diags = append(diags, domain.Warning(
			"spec.runtime.target",
			"runtime target is empty",
			"set runtime.target to an environment/workspace identifier (for example: env/dev)",
		))
	}
	if c.Spec.Runtime.BackendConfig.Compose != nil {
		diags = append(diags, domain.Warning(
			"spec.runtime.backendConfig.compose",
			"compose backendConfig is ignored by terraform adapter",
			"remove compose config or switch to docker-compose backend",
		))
	}
	if c.Spec.Runtime.BackendConfig.SSHSystemd != nil {
		diags = append(diags, domain.Warning(
			"spec.runtime.backendConfig.sshSystemd",
			"sshSystemd backendConfig is ignored by terraform adapter",
			"keep SSH settings in ansible/ssh-systemd workflows",
		))
	}

	return diags
}

// BuildDesired generates deterministic terraform artifacts without runtime execution.
func (b *Backend) BuildDesired(ctx context.Context, c *v1alpha1.ChainCluster, pluginOut chain.Output) (domain.DesiredState, error) {
	_ = ctx

	target := strings.TrimSpace(c.Spec.Runtime.Target)
	if target == "" {
		target = "default"
	}

	nodePools := summarizeNodePools(c.Spec.NodePools)
	tfvars, err := renderTFVarsJSON(c, target, nodePools)
	if err != nil {
		return domain.DesiredState{}, fmt.Errorf("render terraform tfvars: %w", err)
	}

	artifacts := []domain.Artifact{
		{Path: filepath.ToSlash(filepath.Join(moduleDir, "main.tf")), Content: renderMainTF()},
		{Path: filepath.ToSlash(filepath.Join(moduleDir, "variables.tf")), Content: renderVariablesTF()},
		{Path: filepath.ToSlash(filepath.Join(moduleDir, "outputs.tf")), Content: renderOutputsTF()},
		{Path: filepath.ToSlash(filepath.Join(moduleDir, "terraform.tfvars.json")), Content: tfvars},
	}
	artifacts = append(artifacts, pluginOut.Artifacts...)
	sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].Path < artifacts[j].Path })

	return domain.DesiredState{
		ClusterName: c.Metadata.Name,
		Backend:     b.Name(),
		Artifacts:   artifacts,
		Metadata: map[string]string{
			"terraform.module_dir": moduleDir,
			"terraform.var_file":   filepath.ToSlash(filepath.Join(moduleDir, "terraform.tfvars.json")),
			"terraform.target":     target,
		},
	}, nil
}

type tfNodePool struct {
	Name      string       `json:"name"`
	Replicas  int          `json:"replicas"`
	Roles     []string     `json:"roles,omitempty"`
	Workloads []tfWorkload `json:"workloads"`
}

type tfWorkload struct {
	Name string `json:"name"`
	Mode string `json:"mode"`
}

func summarizeNodePools(pools []v1alpha1.NodePoolSpec) []tfNodePool {
	out := make([]tfNodePool, 0, len(pools))
	for _, pool := range pools {
		roles := append([]string{}, pool.Roles...)
		sort.Strings(roles)

		workloads := make([]tfWorkload, 0, len(pool.Template.Workloads))
		for _, w := range pool.Template.Workloads {
			workloads = append(workloads, tfWorkload{Name: w.Name, Mode: string(w.Mode)})
		}
		sort.Slice(workloads, func(i, j int) bool { return workloads[i].Name < workloads[j].Name })

		out = append(out, tfNodePool{
			Name:      pool.Name,
			Replicas:  pool.Replicas,
			Roles:     roles,
			Workloads: workloads,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func renderTFVarsJSON(c *v1alpha1.ChainCluster, target string, pools []tfNodePool) (string, error) {
	payload := map[string]any{
		"cluster_name":    c.Metadata.Name,
		"chain_family":    c.Spec.Family,
		"chain_profile":   c.Spec.Profile,
		"plugin":          c.Spec.Plugin,
		"runtime_backend": c.Spec.Runtime.Backend,
		"runtime_target":  target,
		"node_pools":      pools,
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", err
	}
	return string(raw) + "\n", nil
}

func renderMainTF() string {
	return strings.TrimSpace(`
terraform {
  required_version = ">= 1.5.0"
}

locals {
  cluster_name    = var.cluster_name
  chain_family    = var.chain_family
  chain_profile   = var.chain_profile
  plugin          = var.plugin
  runtime_backend = var.runtime_backend
  runtime_target  = var.runtime_target
  node_pools      = var.node_pools
}
`) + "\n"
}

func renderVariablesTF() string {
	return strings.TrimSpace(`
variable "cluster_name" {
  type        = string
  description = "Logical cluster identifier"
}

variable "chain_family" {
  type        = string
  description = "Portable chain family identifier"
}

variable "chain_profile" {
  type        = string
  description = "Selected chain profile"
}

variable "plugin" {
  type        = string
  description = "Plugin selected by chainops"
}

variable "runtime_backend" {
  type        = string
  description = "Runtime backend selected by chainops"
}

variable "runtime_target" {
  type        = string
  description = "Environment/workspace target for infra deployment"
}

variable "node_pools" {
  type = list(object({
    name     = string
    replicas = number
    roles    = optional(list(string), [])
    workloads = list(object({
      name = string
      mode = string
    }))
  }))
  description = "Portable node topology summary generated by chainops"
}
`) + "\n"
}

func renderOutputsTF() string {
	return strings.TrimSpace(`
output "cluster_name" {
  value       = local.cluster_name
  description = "Cluster identifier"
}

output "runtime_target" {
  value       = local.runtime_target
  description = "Terraform runtime target context"
}

output "node_pool_names" {
  value       = [for pool in local.node_pools : pool.name]
  description = "Node pools present in the rendered topology"
}
`) + "\n"
}
