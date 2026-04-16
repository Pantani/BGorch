package ansible

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Pantani/gorchestrator/internal/api/v1alpha1"
	"github.com/Pantani/gorchestrator/internal/backend"
	"github.com/Pantani/gorchestrator/internal/chain"
	"github.com/Pantani/gorchestrator/internal/domain"
	"github.com/Pantani/gorchestrator/internal/spec"
)

const (
	// BackendName is the canonical registry key for the ansible adapter.
	BackendName = "ansible"
	baseDir     = "ansible"
)

// Backend renders ansible bootstrap artifacts from portable cluster specs.
type Backend struct{}

var _ backend.Backend = (*Backend)(nil)

// New returns an ansible backend instance.
func New() *Backend {
	return &Backend{}
}

// Name returns the canonical backend name.
func (b *Backend) Name() string {
	return BackendName
}

// ValidateTarget validates ansible adapter constraints and emits actionable diagnostics.
func (b *Backend) ValidateTarget(c *v1alpha1.ChainCluster) []domain.Diagnostic {
	diags := make([]domain.Diagnostic, 0)

	if strings.TrimSpace(c.Spec.Runtime.Backend) != BackendName {
		diags = append(diags, domain.Error(
			"spec.runtime.backend",
			"ansible backend selected with incompatible backend name",
			"use spec.runtime.backend: ansible",
		))
		return diags
	}
	if len(c.Spec.NodePools) == 0 {
		diags = append(diags, domain.Error(
			"spec.nodePools",
			"ansible backend requires at least one nodePool",
			"define nodePools to generate host/bootstrap inventory",
		))
	}

	targets := parseTargets(c.Spec.Runtime.Target)
	if strings.TrimSpace(c.Spec.Runtime.Target) != "" && len(targets) == 0 {
		diags = append(diags, domain.Error(
			"spec.runtime.target",
			"runtime target does not contain valid hosts",
			"use comma-separated hosts (for example: node-1.example.com,node-2.example.com)",
		))
	}
	if len(targets) == 0 {
		diags = append(diags, domain.Warning(
			"spec.runtime.target",
			"runtime target is empty",
			"inventory will default to expanded node names; set explicit hosts for remote bootstrap",
		))
	}

	if c.Spec.Runtime.BackendConfig.Compose != nil {
		diags = append(diags, domain.Warning(
			"spec.runtime.backendConfig.compose",
			"compose backendConfig is ignored by ansible adapter",
			"remove compose config or switch to docker-compose backend",
		))
	}
	if sshCfg := c.Spec.Runtime.BackendConfig.SSHSystemd; sshCfg != nil {
		if sshCfg.Port < 0 || sshCfg.Port > 65535 {
			diags = append(diags, domain.Error(
				"spec.runtime.backendConfig.sshSystemd.port",
				"invalid SSH port for ansible connection",
				"use a port in range 1-65535",
			))
		}
	}

	for i, pool := range c.Spec.NodePools {
		for j, w := range pool.Template.Workloads {
			if w.Mode == v1alpha1.WorkloadModeContainer {
				diags = append(diags, domain.Warning(
					fmt.Sprintf("spec.nodePools[%d].template.workloads[%d].mode", i, j),
					"container workload detected in ansible backend",
					"ansible adapter focuses on host bootstrap; container workloads are treated as metadata",
				))
			}
		}
	}

	return diags
}

// BuildDesired renders deterministic ansible inventory/playbook artifacts.
func (b *Backend) BuildDesired(ctx context.Context, c *v1alpha1.ChainCluster, pluginOut chain.Output) (domain.DesiredState, error) {
	_ = ctx

	nodes := spec.ExpandNodes(c)
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Name < nodes[j].Name })

	hosts := parseTargets(c.Spec.Runtime.Target)
	if len(hosts) == 0 {
		hosts = make([]string, 0, len(nodes))
		for _, n := range nodes {
			hosts = append(hosts, n.Name)
		}
	}
	hosts = uniqueSorted(hosts)

	sshUser := "root"
	sshPort := 22
	if sshCfg := c.Spec.Runtime.BackendConfig.SSHSystemd; sshCfg != nil {
		if strings.TrimSpace(sshCfg.User) != "" {
			sshUser = strings.TrimSpace(sshCfg.User)
		}
		if sshCfg.Port > 0 {
			sshPort = sshCfg.Port
		}
	}

	services := summarizeServices(c.Metadata.Name, nodes)
	unitNames := make([]string, 0, len(services))
	for _, svc := range services {
		unitNames = append(unitNames, svc.Name+".service")
	}
	sort.Strings(unitNames)

	artifacts := []domain.Artifact{
		{
			Path:    filepath.ToSlash(filepath.Join(baseDir, "inventory.ini")),
			Content: renderInventory(hosts, sshUser, sshPort),
		},
		{
			Path:    filepath.ToSlash(filepath.Join(baseDir, "group_vars", "all.yml")),
			Content: renderGroupVars(c.Metadata.Name, c.Spec.Family, c.Spec.Plugin, unitNames),
		},
		{
			Path:    filepath.ToSlash(filepath.Join(baseDir, "playbook.bootstrap.yml")),
			Content: renderPlaybook(),
		},
	}
	artifacts = append(artifacts, pluginOut.Artifacts...)
	sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].Path < artifacts[j].Path })

	return domain.DesiredState{
		ClusterName: c.Metadata.Name,
		Backend:     b.Name(),
		Services:    services,
		Artifacts:   artifacts,
		Metadata: map[string]string{
			"ansible.inventory": filepath.ToSlash(filepath.Join(baseDir, "inventory.ini")),
			"ansible.playbook":  filepath.ToSlash(filepath.Join(baseDir, "playbook.bootstrap.yml")),
			"ansible.user":      sshUser,
			"ansible.port":      strconv.Itoa(sshPort),
		},
	}, nil
}

func summarizeServices(clusterName string, nodes []spec.ResolvedNode) []domain.Service {
	services := make([]domain.Service, 0)
	for _, n := range nodes {
		workloads := append([]v1alpha1.WorkloadSpec{}, n.Spec.Workloads...)
		sort.Slice(workloads, func(i, j int) bool { return workloads[i].Name < workloads[j].Name })

		for _, w := range workloads {
			svc := domain.Service{
				Name:          serviceName(clusterName, n.Name, w.Name),
				Node:          n.Name,
				Workload:      w.Name,
				Image:         w.Image,
				Command:       resolveCommand(w),
				Args:          append([]string{}, w.Args...),
				Env:           envMap(w.Env),
				RestartPolicy: string(w.RestartPolicy),
			}
			services = append(services, svc)
		}
	}
	sort.Slice(services, func(i, j int) bool { return services[i].Name < services[j].Name })
	return services
}

func parseTargets(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if host := strings.TrimSpace(p); host != "" {
			out = append(out, host)
		}
	}
	return out
}

func uniqueSorted(in []string) []string {
	set := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, item := range in {
		if _, ok := set[item]; ok {
			continue
		}
		set[item] = struct{}{}
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}

func serviceName(cluster, node, workload string) string {
	return sanitizeName(cluster) + "-" + sanitizeName(node) + "-" + sanitizeName(workload)
}

func sanitizeName(in string) string {
	in = strings.ToLower(strings.TrimSpace(in))
	if in == "" {
		return "x"
	}
	var b strings.Builder
	lastDash := false
	for _, r := range in {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteRune('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "x"
	}
	return out
}

func resolveCommand(w v1alpha1.WorkloadSpec) []string {
	if len(w.Command) > 0 {
		return append([]string{}, w.Command...)
	}
	if strings.TrimSpace(w.Binary) != "" {
		return []string{strings.TrimSpace(w.Binary)}
	}
	return nil
}

func envMap(env []v1alpha1.EnvVar) map[string]string {
	if len(env) == 0 {
		return nil
	}
	out := make(map[string]string, len(env))
	for _, e := range env {
		out[e.Name] = e.Value
	}
	return out
}

func renderInventory(hosts []string, sshUser string, sshPort int) string {
	var b strings.Builder
	b.WriteString("[chainops_targets]\n")
	for i, host := range hosts {
		alias := fmt.Sprintf("host_%02d", i+1)
		b.WriteString(alias)
		b.WriteString(" ansible_host=")
		b.WriteString(host)
		b.WriteString(" ansible_user=")
		b.WriteString(sshUser)
		b.WriteString(" ansible_port=")
		b.WriteString(strconv.Itoa(sshPort))
		b.WriteString("\n")
	}
	return b.String()
}

func renderGroupVars(clusterName, family, plugin string, units []string) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("chainops_cluster_name: ")
	b.WriteString(clusterName)
	b.WriteString("\n")
	b.WriteString("chainops_family: ")
	b.WriteString(family)
	b.WriteString("\n")
	b.WriteString("chainops_plugin: ")
	b.WriteString(plugin)
	b.WriteString("\n")
	b.WriteString("chainops_render_src: \"../render\"\n")
	b.WriteString("chainops_render_dst: \"/etc/chainops/{{ chainops_cluster_name }}/render\"\n")
	b.WriteString("chainops_systemd_units:\n")
	for _, unit := range units {
		b.WriteString("  - ")
		b.WriteString(unit)
		b.WriteString("\n")
	}
	return b.String()
}

func renderPlaybook() string {
	return strings.TrimSpace(`
---
- name: Chainops host bootstrap
  hosts: chainops_targets
  become: true
  gather_facts: true

  tasks:
    - name: Ensure chainops directories exist
      ansible.builtin.file:
        path: "{{ item }}"
        state: directory
        mode: "0755"
      loop:
        - "/etc/chainops/{{ chainops_cluster_name }}"
        - "/var/lib/chainops/{{ chainops_cluster_name }}"

    - name: Sync rendered configuration bundle
      ansible.builtin.copy:
        src: "{{ chainops_render_src }}/"
        dest: "{{ chainops_render_dst }}/"
        mode: "0644"

    - name: Install systemd units when available
      ansible.builtin.copy:
        src: "{{ chainops_render_dst }}/ssh-systemd/nodes/{{ inventory_hostname }}/systemd/{{ item }}"
        dest: "/etc/systemd/system/{{ item }}"
        mode: "0644"
      loop: "{{ chainops_systemd_units }}"
      ignore_errors: true

    - name: Reload systemd daemon
      ansible.builtin.systemd:
        daemon_reload: true

    - name: Ensure chainops units are enabled and started
      ansible.builtin.systemd:
        name: "{{ item }}"
        enabled: true
        state: started
      loop: "{{ chainops_systemd_units }}"
      ignore_errors: true
`) + "\n"
}
