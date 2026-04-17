package solana

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Pantani/gorchestrator/internal/api/v1alpha1"
	"github.com/Pantani/gorchestrator/internal/chain"
	"github.com/Pantani/gorchestrator/internal/domain"
	"github.com/Pantani/gorchestrator/internal/spec"
)

const (
	// PluginName is the registry identifier for the solana plugin.
	PluginName = "solana-family"
	// FamilyName is the normalized chain family handled by this plugin.
	FamilyName = "solana"
)

var _ chain.Plugin = (*Plugin)(nil)

// Plugin implements chain.Plugin for Solana defaults and artifacts.
type Plugin struct{}

// New returns a solana-family plugin instance.
func New() *Plugin {
	return &Plugin{}
}

func (p *Plugin) Name() string {
	return PluginName
}

func (p *Plugin) Family() string {
	return FamilyName
}

func (p *Plugin) Capabilities() chain.Capabilities {
	return chain.Capabilities{
		SupportsMultiProcess: true,
		SupportsBootstrap:    true,
		SupportsBackup:       true,
		SupportsRestore:      true,
		SupportsUpgrade:      true,
	}
}

// Validate checks solana-family assumptions around workloads and common ports.
func (p *Plugin) Validate(c *v1alpha1.ChainCluster) []domain.Diagnostic {
	diags := make([]domain.Diagnostic, 0)

	if strings.TrimSpace(c.Spec.Plugin) != PluginName {
		diags = append(diags, domain.Error(
			"spec.plugin",
			"solana-family plugin selected with mismatched plugin name",
			"set spec.plugin to solana-family",
		))
	}

	if strings.TrimSpace(c.Spec.Family) == "" {
		diags = append(diags, domain.Error(
			"spec.family",
			"family must be set",
			"set family to solana",
		))
	} else if normalizeFamily(c.Spec.Family) != FamilyName {
		diags = append(diags, domain.Warning(
			"spec.family",
			"plugin is optimized for family solana",
			"prefer family: solana",
		))
	}

	for i, pool := range c.Spec.NodePools {
		poolPath := fmt.Sprintf("spec.nodePools[%d].template", i)
		workloadIndex, workload := findSolanaWorkload(pool.Template.Workloads)
		hasTypedClient := false
		if pool.Template.PluginConfig.Solana != nil && strings.TrimSpace(pool.Template.PluginConfig.Solana.Client) != "" {
			hasTypedClient = true
		}
		if !hasTypedClient {
			for _, w := range pool.Template.Workloads {
				if w.PluginConfig.Solana != nil && strings.TrimSpace(w.PluginConfig.Solana.Client) != "" {
					hasTypedClient = true
					break
				}
			}
		}
		if workloadIndex < 0 && !hasTypedClient {
			diags = append(diags, domain.Error(
				poolPath+".workloads",
				"solana-family requires at least one Solana workload per node template",
				"define one workload with solana-validator/agave/jito command or image",
			))
			continue
		}

		if workload == nil {
			continue
		}
		if !hasPort(workload.Ports, 8899) {
			diags = append(diags, domain.Warning(
				fmt.Sprintf("%s.workloads[%d].ports", poolPath, workloadIndex),
				"rpc port 8899 not declared",
				"declare containerPort 8899 for JSON-RPC",
			))
		}
		if !hasPort(workload.Ports, 8001) {
			diags = append(diags, domain.Warning(
				fmt.Sprintf("%s.workloads[%d].ports", poolPath, workloadIndex),
				"gossip port 8001 not declared",
				"declare containerPort 8001 for gossip connectivity",
			))
		}
	}

	return diags
}

// Normalize infers default family/profile and canonical config formatting.
func (p *Plugin) Normalize(c *v1alpha1.ChainCluster) error {
	c.Spec.Family = FamilyName
	if strings.TrimSpace(c.Spec.Profile) == "" {
		c.Spec.Profile = "solana-rpc"
	}
	normalizeSolanaConfig(c.Spec.PluginConfig.Solana)
	for i := range c.Spec.NodePools {
		pool := &c.Spec.NodePools[i]
		normalizeSolanaConfig(pool.Template.PluginConfig.Solana)
		for j := range pool.Template.Workloads {
			normalizeSolanaConfig(pool.Template.Workloads[j].PluginConfig.Solana)
		}
	}
	return nil
}

// Build renders Solana node env artifacts with deterministic ordering.
func (p *Plugin) Build(ctx context.Context, c *v1alpha1.ChainCluster) (chain.Output, error) {
	_ = ctx

	artifactsByPath := make(map[string]string)
	nodes := spec.ExpandNodes(c)
	for _, n := range nodes {
		workloadIndex, workload := findSolanaWorkload(n.Spec.Workloads)
		var workloadCfg *v1alpha1.SolanaConfig
		if workloadIndex >= 0 {
			workloadCfg = n.Spec.Workloads[workloadIndex].PluginConfig.Solana
		}

		resolved := resolveSolanaConfig(
			workload,
			c.Spec.PluginConfig.Solana,
			n.Spec.PluginConfig.Solana,
			workloadCfg,
		)
		addArtifact(artifactsByPath, filepath.Join("nodes", n.Name, "config", "solana.env"), renderEnvFile(resolved))
		for _, f := range n.Spec.Files {
			addArtifact(artifactsByPath, filepath.Join("nodes", n.Name, filepath.Clean(f.Path)), f.Content)
		}
	}

	paths := make([]string, 0, len(artifactsByPath))
	for path := range artifactsByPath {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	artifacts := make([]domain.Artifact, 0, len(paths))
	for _, path := range paths {
		artifacts = append(artifacts, domain.Artifact{Path: path, Content: artifactsByPath[path]})
	}

	return chain.Output{
		Artifacts: artifacts,
		Metadata: map[string]string{
			"plugin":  p.Name(),
			"family":  c.Spec.Family,
			"profile": c.Spec.Profile,
		},
	}, nil
}

type resolvedSolanaConfig struct {
	Client              string
	Cluster             string
	RPCPort             int
	WSPort              int
	GossipPort          int
	DynamicPortRange    string
	EntryPoints         string
	FullRPC             bool
	PrivateRPC          bool
	NoVoting            bool
	ExpectedGenesisHash string
}

func resolveSolanaConfig(
	workload *v1alpha1.WorkloadSpec,
	clusterCfg *v1alpha1.SolanaConfig,
	nodeCfg *v1alpha1.SolanaConfig,
	workloadCfg *v1alpha1.SolanaConfig,
) resolvedSolanaConfig {
	merged := mergeSolanaConfig(clusterCfg, nodeCfg, workloadCfg)

	client := inferClientFromConfigOrWorkload(merged.Client, workload)
	rpcPort := 8899
	wsPort := 8900
	gossipPort := 8001
	if workload != nil {
		rpcPort = firstPortByNameOrDefault(workload.Ports, []string{"rpc", "json-rpc"}, rpcPort)
		wsPort = firstPortByNameOrDefault(workload.Ports, []string{"ws", "websocket"}, wsPort)
		gossipPort = firstPortByNameOrDefault(workload.Ports, []string{"gossip", "p2p"}, gossipPort)
	}

	cfg := resolvedSolanaConfig{
		Client:              client,
		Cluster:             "mainnet-beta",
		RPCPort:             rpcPort,
		WSPort:              wsPort,
		GossipPort:          gossipPort,
		DynamicPortRange:    "8000-8020",
		EntryPoints:         strings.Join(merged.EntryPoints, ","),
		FullRPC:             true,
		PrivateRPC:          false,
		NoVoting:            false,
		ExpectedGenesisHash: merged.ExpectedGenesisHash,
	}

	if merged.Cluster != "" {
		cfg.Cluster = merged.Cluster
	}
	if merged.RPCPort > 0 {
		cfg.RPCPort = merged.RPCPort
	}
	if merged.WSPort > 0 {
		cfg.WSPort = merged.WSPort
	}
	if merged.GossipPort > 0 {
		cfg.GossipPort = merged.GossipPort
	}
	if merged.DynamicPortRange != "" {
		cfg.DynamicPortRange = merged.DynamicPortRange
	}
	if merged.FullRPC != nil {
		cfg.FullRPC = *merged.FullRPC
	}
	if merged.PrivateRPC != nil {
		cfg.PrivateRPC = *merged.PrivateRPC
	}
	if merged.NoVoting != nil {
		cfg.NoVoting = *merged.NoVoting
	}

	return cfg
}

func renderEnvFile(cfg resolvedSolanaConfig) string {
	var b strings.Builder
	b.WriteString("# Generated by bgorch solana-family plugin\n")
	b.WriteString("SOLANA_CLIENT=" + cfg.Client + "\n")
	b.WriteString("SOLANA_CLUSTER=" + cfg.Cluster + "\n")
	b.WriteString("SOLANA_RPC_PORT=" + strconv.Itoa(cfg.RPCPort) + "\n")
	b.WriteString("SOLANA_WS_PORT=" + strconv.Itoa(cfg.WSPort) + "\n")
	b.WriteString("SOLANA_GOSSIP_PORT=" + strconv.Itoa(cfg.GossipPort) + "\n")
	b.WriteString("SOLANA_DYNAMIC_PORT_RANGE=" + cfg.DynamicPortRange + "\n")
	b.WriteString("SOLANA_ENTRYPOINTS=" + cfg.EntryPoints + "\n")
	b.WriteString("SOLANA_FULL_RPC=" + strconv.FormatBool(cfg.FullRPC) + "\n")
	b.WriteString("SOLANA_PRIVATE_RPC=" + strconv.FormatBool(cfg.PrivateRPC) + "\n")
	b.WriteString("SOLANA_NO_VOTING=" + strconv.FormatBool(cfg.NoVoting) + "\n")
	b.WriteString("SOLANA_EXPECTED_GENESIS_HASH=" + cfg.ExpectedGenesisHash + "\n")
	return b.String()
}

func mergeSolanaConfig(configs ...*v1alpha1.SolanaConfig) v1alpha1.SolanaConfig {
	var out v1alpha1.SolanaConfig
	for _, cfg := range configs {
		if cfg == nil {
			continue
		}
		if cfg.Client != "" {
			out.Client = cfg.Client
		}
		if cfg.Cluster != "" {
			out.Cluster = cfg.Cluster
		}
		if cfg.RPCPort > 0 {
			out.RPCPort = cfg.RPCPort
		}
		if cfg.WSPort > 0 {
			out.WSPort = cfg.WSPort
		}
		if cfg.GossipPort > 0 {
			out.GossipPort = cfg.GossipPort
		}
		if cfg.DynamicPortRange != "" {
			out.DynamicPortRange = cfg.DynamicPortRange
		}
		if len(cfg.EntryPoints) > 0 {
			out.EntryPoints = filterNonEmpty(cfg.EntryPoints)
		}
		if cfg.FullRPC != nil {
			v := *cfg.FullRPC
			out.FullRPC = &v
		}
		if cfg.PrivateRPC != nil {
			v := *cfg.PrivateRPC
			out.PrivateRPC = &v
		}
		if cfg.NoVoting != nil {
			v := *cfg.NoVoting
			out.NoVoting = &v
		}
		if cfg.ExpectedGenesisHash != "" {
			out.ExpectedGenesisHash = strings.TrimSpace(cfg.ExpectedGenesisHash)
		}
	}
	return out
}

func normalizeSolanaConfig(cfg *v1alpha1.SolanaConfig) {
	if cfg == nil {
		return
	}
	cfg.Client = normalizeClient(cfg.Client)
	cfg.Cluster = normalizeCluster(cfg.Cluster)
	cfg.DynamicPortRange = strings.TrimSpace(cfg.DynamicPortRange)
	cfg.EntryPoints = filterNonEmpty(cfg.EntryPoints)
	cfg.ExpectedGenesisHash = strings.TrimSpace(cfg.ExpectedGenesisHash)
}

func inferClientFromConfigOrWorkload(configured string, workload *v1alpha1.WorkloadSpec) string {
	if configured = normalizeClient(configured); configured != "" {
		return configured
	}
	if workload != nil {
		if client := inferClient(*workload); client != "" {
			return client
		}
	}
	return "agave"
}

func inferClient(workload v1alpha1.WorkloadSpec) string {
	parts := []string{workload.Name, workload.Image, workload.Binary}
	parts = append(parts, workload.Command...)
	parts = append(parts, workload.Args...)
	for _, part := range parts {
		value := strings.ToLower(part)
		switch {
		case strings.Contains(value, "jito"):
			return "jito"
		case strings.Contains(value, "solana-validator"), strings.Contains(value, "agave"), strings.Contains(value, "solana"):
			return "agave"
		}
	}
	return ""
}

func findSolanaWorkload(workloads []v1alpha1.WorkloadSpec) (int, *v1alpha1.WorkloadSpec) {
	for i := range workloads {
		if isSolanaWorkload(workloads[i]) {
			return i, &workloads[i]
		}
	}
	return -1, nil
}

func isSolanaWorkload(w v1alpha1.WorkloadSpec) bool {
	return inferClient(w) != ""
}

func firstPortByNameOrDefault(ports []v1alpha1.PortSpec, names []string, fallback int) int {
	if len(ports) == 0 {
		return fallback
	}
	nameSet := make(map[string]struct{}, len(names))
	for _, name := range names {
		nameSet[strings.ToLower(strings.TrimSpace(name))] = struct{}{}
	}
	for _, p := range ports {
		if _, ok := nameSet[strings.ToLower(strings.TrimSpace(p.Name))]; ok && p.ContainerPort > 0 {
			return p.ContainerPort
		}
	}
	return fallback
}

func hasPort(ports []v1alpha1.PortSpec, want int) bool {
	for _, p := range ports {
		if p.ContainerPort == want {
			return true
		}
	}
	return false
}

func normalizeFamily(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == FamilyName {
		return FamilyName
	}
	return ""
}

func normalizeClient(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "solana-validator":
		return "agave"
	case "agave", "jito":
		return value
	default:
		return value
	}
}

func normalizeCluster(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "mainnet-beta", "testnet", "devnet", "localnet":
		return value
	default:
		return value
	}
}

func filterNonEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func addArtifact(dst map[string]string, path, content string) {
	dst[safeRelPath(path)] = content
}

func safeRelPath(path string) string {
	path = strings.TrimPrefix(filepath.ToSlash(filepath.Clean(path)), "/")
	for strings.HasPrefix(path, "../") {
		path = strings.TrimPrefix(path, "../")
	}
	if path == ".." || path == "." || path == "" {
		return "artifact.txt"
	}
	return path
}
