package kubernetes

import (
	"context"
	"fmt"
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
	BackendName         = "kubernetes"
	defaultNamespace    = "default"
	defaultManifestFile = "kubernetes/manifests.yaml"
	defaultStorageSize  = "20Gi"
)

type Backend struct{}

var _ backend.Backend = (*Backend)(nil)

func New() *Backend {
	return &Backend{}
}

func (b *Backend) Name() string {
	return BackendName
}

func (b *Backend) ValidateTarget(c *v1alpha1.ChainCluster) []domain.Diagnostic {
	diags := make([]domain.Diagnostic, 0)
	if strings.TrimSpace(c.Spec.Runtime.Backend) != BackendName {
		diags = append(diags, domain.Error(
			"spec.runtime.backend",
			"kubernetes backend selected with incompatible backend name",
			"use kubernetes",
		))
		return diags
	}
	if len(c.Spec.NodePools) == 0 {
		diags = append(diags, domain.Error(
			"spec.nodePools",
			"kubernetes backend requires at least one nodePool",
			"define spec.nodePools with at least one workload",
		))
		return diags
	}

	for i, pool := range c.Spec.NodePools {
		for j, w := range pool.Template.Workloads {
			path := fmt.Sprintf("spec.nodePools[%d].template.workloads[%d]", i, j)
			if w.Mode == v1alpha1.WorkloadModeHost {
				diags = append(diags, domain.Error(
					path+".mode",
					"kubernetes backend only supports container mode workloads",
					"set mode: container or choose ssh-systemd backend",
				))
			}
			if strings.TrimSpace(w.Image) == "" {
				diags = append(diags, domain.Error(
					path+".image",
					"kubernetes backend requires workload.image",
					"set a container image for each workload",
				))
			}
			if len(w.Ports) == 0 {
				diags = append(diags, domain.Error(
					path+".ports",
					"kubernetes backend requires at least one port to render Service",
					"define workload.ports with containerPort entries",
				))
			}
		}
	}

	return diags
}

func (b *Backend) BuildDesired(ctx context.Context, c *v1alpha1.ChainCluster, pluginOut chain.Output) (domain.DesiredState, error) {
	_ = ctx

	namespace := sanitizeName(c.Spec.Runtime.Target)
	if namespace == "" || namespace == "unnamed" {
		namespace = defaultNamespace
	}

	nodes := spec.ExpandNodes(c)
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Name < nodes[j].Name })

	services := make([]domain.Service, 0)
	volumesByName := make(map[string]domain.Volume)
	resources := make([]workloadResource, 0)

	for _, n := range nodes {
		volumeDefs := make(map[string]v1alpha1.VolumeSpec, len(n.Spec.Volumes))
		for _, vol := range n.Spec.Volumes {
			volumeDefs[vol.Name] = vol
		}

		workloads := append([]v1alpha1.WorkloadSpec{}, n.Spec.Workloads...)
		sort.Slice(workloads, func(i, j int) bool { return workloads[i].Name < workloads[j].Name })

		for _, w := range workloads {
			if w.Mode == v1alpha1.WorkloadModeHost {
				return domain.DesiredState{}, fmt.Errorf("workload %s/%s is host mode and cannot run on kubernetes backend", n.Name, w.Name)
			}
			if strings.TrimSpace(w.Image) == "" {
				return domain.DesiredState{}, fmt.Errorf("workload %s/%s requires image for kubernetes backend", n.Name, w.Name)
			}
			if len(w.Ports) == 0 {
				return domain.DesiredState{}, fmt.Errorf("workload %s/%s requires at least one port for kubernetes Service", n.Name, w.Name)
			}

			svcName := serviceName(c.Metadata.Name, n.Name, w.Name)
			svc := domain.Service{
				Name:          svcName,
				Node:          n.Name,
				Workload:      w.Name,
				Image:         strings.TrimSpace(w.Image),
				Command:       append([]string{}, w.Command...),
				Args:          append([]string{}, w.Args...),
				Env:           envMap(w.Env),
				RestartPolicy: string(w.RestartPolicy),
			}

			for _, p := range w.Ports {
				svc.Ports = append(svc.Ports, domain.PortBinding{
					ContainerPort: p.ContainerPort,
					HostPort:      p.HostPort,
					Protocol:      normalizeProtocol(p.Protocol),
				})
			}
			sort.Slice(svc.Ports, func(i, j int) bool {
				if svc.Ports[i].ContainerPort == svc.Ports[j].ContainerPort {
					return svc.Ports[i].HostPort < svc.Ports[j].HostPort
				}
				return svc.Ports[i].ContainerPort < svc.Ports[j].ContainerPort
			})

			claims := make([]claimTemplate, 0)
			claimsByName := make(map[string]claimTemplate)
			for _, mount := range w.VolumeMounts {
				vol, ok := volumeDefs[mount.Volume]
				if !ok {
					return domain.DesiredState{}, fmt.Errorf("volume %q not found for workload %s/%s", mount.Volume, n.Name, w.Name)
				}
				claim := claimTemplate{
					Name:    sanitizeName(vol.Name),
					Storage: defaultStorageSize,
				}
				if claim.Name == "" || claim.Name == "unnamed" {
					claim.Name = "data"
				}
				if vol.Type == v1alpha1.VolumeTypeBind {
					claim.SourceType = string(v1alpha1.VolumeTypeBind)
					claim.SourcePath = strings.TrimSpace(vol.Source)
				} else {
					claim.SourceType = string(v1alpha1.VolumeTypeNamed)
				}
				if _, exists := claimsByName[claim.Name]; !exists {
					claimsByName[claim.Name] = claim
					claims = append(claims, claim)
				}

				svc.Volumes = append(svc.Volumes, domain.VolumeMount{
					Source:   claim.Name,
					Target:   mount.Path,
					Type:     "persistentVolumeClaim",
					ReadOnly: mount.ReadOnly,
				})
			}
			sort.Slice(svc.Volumes, func(i, j int) bool {
				if svc.Volumes[i].Source == svc.Volumes[j].Source {
					return svc.Volumes[i].Target < svc.Volumes[j].Target
				}
				return svc.Volumes[i].Source < svc.Volumes[j].Source
			})
			sort.Slice(claims, func(i, j int) bool { return claims[i].Name < claims[j].Name })

			for _, claim := range claims {
				volName := sanitizeName(fmt.Sprintf("%s-%s", svcName, claim.Name))
				volumesByName[volName] = domain.Volume{Name: volName}
			}

			services = append(services, svc)
			resources = append(resources, workloadResource{
				Service: svc,
				Claims:  claims,
			})
		}
	}

	sort.Slice(services, func(i, j int) bool { return services[i].Name < services[j].Name })
	sort.Slice(resources, func(i, j int) bool { return resources[i].Service.Name < resources[j].Service.Name })

	volumes := make([]domain.Volume, 0, len(volumesByName))
	for _, vol := range volumesByName {
		volumes = append(volumes, vol)
	}
	sort.Slice(volumes, func(i, j int) bool { return volumes[i].Name < volumes[j].Name })

	desired := domain.DesiredState{
		ClusterName: c.Metadata.Name,
		Backend:     b.Name(),
		Services:    services,
		Volumes:     volumes,
		Metadata: map[string]string{
			"kubernetes.namespace": namespace,
			"kubernetes.file":      defaultManifestFile,
		},
	}

	manifest := renderManifest(c.Metadata.Name, namespace, resources)
	desired.Artifacts = append(desired.Artifacts, domain.Artifact{Path: defaultManifestFile, Content: manifest})
	desired.Artifacts = append(desired.Artifacts, pluginOut.Artifacts...)
	sort.Slice(desired.Artifacts, func(i, j int) bool { return desired.Artifacts[i].Path < desired.Artifacts[j].Path })

	return desired, nil
}

type workloadResource struct {
	Service domain.Service
	Claims  []claimTemplate
}

type claimTemplate struct {
	Name       string
	Storage    string
	SourceType string
	SourcePath string
}

func renderManifest(clusterName, namespace string, resources []workloadResource) string {
	docs := make([]string, 0, len(resources)*2)
	for _, res := range resources {
		docs = append(docs, renderService(clusterName, namespace, res.Service))
		docs = append(docs, renderStatefulSet(clusterName, namespace, res.Service, res.Claims))
	}
	return strings.Join(docs, "---\n")
}

func renderService(clusterName, namespace string, svc domain.Service) string {
	var b strings.Builder
	b.WriteString("apiVersion: v1\n")
	b.WriteString("kind: Service\n")
	b.WriteString("metadata:\n")
	b.WriteString("  name: ")
	b.WriteString(svc.Name)
	b.WriteString("\n")
	b.WriteString("  namespace: ")
	b.WriteString(namespace)
	b.WriteString("\n")
	b.WriteString("  labels:\n")
	b.WriteString("    app.kubernetes.io/name: chainops\n")
	b.WriteString("    chainops.io/cluster: ")
	b.WriteString(clusterName)
	b.WriteString("\n")
	b.WriteString("    chainops.io/node: ")
	b.WriteString(svc.Node)
	b.WriteString("\n")
	b.WriteString("    chainops.io/workload: ")
	b.WriteString(svc.Workload)
	b.WriteString("\n")
	b.WriteString("spec:\n")
	b.WriteString("  selector:\n")
	b.WriteString("    chainops.io/service: ")
	b.WriteString(svc.Name)
	b.WriteString("\n")
	b.WriteString("  ports:\n")
	for _, p := range svc.Ports {
		name := sanitizeName(fmt.Sprintf("%d-%s", p.ContainerPort, strings.ToLower(p.Protocol)))
		if name == "" || name == "unnamed" {
			name = "p" + strconv.Itoa(p.ContainerPort)
		}
		b.WriteString("    - name: ")
		b.WriteString(name)
		b.WriteString("\n")
		b.WriteString("      port: ")
		b.WriteString(strconv.Itoa(p.ContainerPort))
		b.WriteString("\n")
		b.WriteString("      targetPort: ")
		b.WriteString(strconv.Itoa(p.ContainerPort))
		b.WriteString("\n")
		b.WriteString("      protocol: ")
		b.WriteString(strings.ToUpper(p.Protocol))
		b.WriteString("\n")
	}
	return b.String()
}

func renderStatefulSet(clusterName, namespace string, svc domain.Service, claims []claimTemplate) string {
	var b strings.Builder
	b.WriteString("apiVersion: apps/v1\n")
	b.WriteString("kind: StatefulSet\n")
	b.WriteString("metadata:\n")
	b.WriteString("  name: ")
	b.WriteString(svc.Name)
	b.WriteString("\n")
	b.WriteString("  namespace: ")
	b.WriteString(namespace)
	b.WriteString("\n")
	b.WriteString("  labels:\n")
	b.WriteString("    app.kubernetes.io/name: chainops\n")
	b.WriteString("    chainops.io/cluster: ")
	b.WriteString(clusterName)
	b.WriteString("\n")
	b.WriteString("    chainops.io/node: ")
	b.WriteString(svc.Node)
	b.WriteString("\n")
	b.WriteString("    chainops.io/workload: ")
	b.WriteString(svc.Workload)
	b.WriteString("\n")
	b.WriteString("spec:\n")
	b.WriteString("  serviceName: ")
	b.WriteString(svc.Name)
	b.WriteString("\n")
	b.WriteString("  replicas: 1\n")
	b.WriteString("  selector:\n")
	b.WriteString("    matchLabels:\n")
	b.WriteString("      chainops.io/service: ")
	b.WriteString(svc.Name)
	b.WriteString("\n")
	b.WriteString("  template:\n")
	b.WriteString("    metadata:\n")
	b.WriteString("      labels:\n")
	b.WriteString("        chainops.io/service: ")
	b.WriteString(svc.Name)
	b.WriteString("\n")
	b.WriteString("        chainops.io/cluster: ")
	b.WriteString(clusterName)
	b.WriteString("\n")
	b.WriteString("        chainops.io/node: ")
	b.WriteString(svc.Node)
	b.WriteString("\n")
	b.WriteString("    spec:\n")
	b.WriteString("      containers:\n")
	b.WriteString("        - name: ")
	b.WriteString(sanitizeName(svc.Workload))
	b.WriteString("\n")
	b.WriteString("          image: ")
	b.WriteString(quote(svc.Image))
	b.WriteString("\n")

	if len(svc.Command) > 0 {
		b.WriteString("          command:\n")
		for _, cmd := range svc.Command {
			b.WriteString("            - ")
			b.WriteString(quote(cmd))
			b.WriteString("\n")
		}
	}
	if len(svc.Args) > 0 {
		b.WriteString("          args:\n")
		for _, arg := range svc.Args {
			b.WriteString("            - ")
			b.WriteString(quote(arg))
			b.WriteString("\n")
		}
	}

	if len(svc.Env) > 0 {
		keys := make([]string, 0, len(svc.Env))
		for k := range svc.Env {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteString("          env:\n")
		for _, k := range keys {
			b.WriteString("            - name: ")
			b.WriteString(k)
			b.WriteString("\n")
			b.WriteString("              value: ")
			b.WriteString(quote(svc.Env[k]))
			b.WriteString("\n")
		}
	}

	if len(svc.Ports) > 0 {
		b.WriteString("          ports:\n")
		for _, p := range svc.Ports {
			name := sanitizeName(fmt.Sprintf("%d-%s", p.ContainerPort, strings.ToLower(p.Protocol)))
			if name == "" || name == "unnamed" {
				name = "p" + strconv.Itoa(p.ContainerPort)
			}
			b.WriteString("            - name: ")
			b.WriteString(name)
			b.WriteString("\n")
			b.WriteString("              containerPort: ")
			b.WriteString(strconv.Itoa(p.ContainerPort))
			b.WriteString("\n")
			b.WriteString("              protocol: ")
			b.WriteString(strings.ToUpper(p.Protocol))
			b.WriteString("\n")
		}
	}

	if len(svc.Volumes) > 0 {
		b.WriteString("          volumeMounts:\n")
		for _, vm := range svc.Volumes {
			b.WriteString("            - name: ")
			b.WriteString(vm.Source)
			b.WriteString("\n")
			b.WriteString("              mountPath: ")
			b.WriteString(vm.Target)
			b.WriteString("\n")
			if vm.ReadOnly {
				b.WriteString("              readOnly: true\n")
			}
		}
	}

	if len(claims) > 0 {
		b.WriteString("  volumeClaimTemplates:\n")
		for _, claim := range claims {
			b.WriteString("    - metadata:\n")
			b.WriteString("        name: ")
			b.WriteString(claim.Name)
			b.WriteString("\n")
			if claim.SourceType == string(v1alpha1.VolumeTypeBind) {
				b.WriteString("        annotations:\n")
				b.WriteString("          chainops.io/source-volume-type: bind\n")
				if claim.SourcePath != "" {
					b.WriteString("          chainops.io/source-path: ")
					b.WriteString(quote(claim.SourcePath))
					b.WriteString("\n")
				}
			}
			b.WriteString("      spec:\n")
			b.WriteString("        accessModes:\n")
			b.WriteString("          - ReadWriteOnce\n")
			b.WriteString("        resources:\n")
			b.WriteString("          requests:\n")
			b.WriteString("            storage: ")
			b.WriteString(claim.Storage)
			b.WriteString("\n")
		}
	}

	return b.String()
}

func serviceName(clusterName, nodeName, workloadName string) string {
	return sanitizeName(fmt.Sprintf("%s-%s-%s", clusterName, nodeName, workloadName))
}

func envMap(values []v1alpha1.EnvVar) map[string]string {
	out := make(map[string]string, len(values))
	for _, e := range values {
		out[e.Name] = e.Value
	}
	return out
}

func normalizeProtocol(v string) string {
	v = strings.TrimSpace(strings.ToUpper(v))
	if v == "" {
		return "TCP"
	}
	if v != "TCP" && v != "UDP" && v != "SCTP" {
		return "TCP"
	}
	return v
}

func sanitizeName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	repl := strings.NewReplacer(" ", "-", "/", "-", "_", "-", ".", "-", ":", "-")
	s = repl.Replace(s)
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return !(r == '-' || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
	})
	s = strings.Join(parts, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "unnamed"
	}
	return s
}

func quote(v string) string {
	return strconv.Quote(v)
}
