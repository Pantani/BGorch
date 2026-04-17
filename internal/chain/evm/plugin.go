package evm

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
	// PluginName is the registry identifier for the evm plugin.
	PluginName = "evm-family"
	// FamilyName is the normalized chain family handled by this plugin.
	FamilyName = "evm"
)

var _ chain.Plugin = (*Plugin)(nil)

// Plugin implements chain.Plugin for EVM-oriented defaults and artifacts.
type Plugin struct{}

// New returns an evm-family plugin instance.
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

// Validate checks evm-family assumptions around workloads and common ports.
func (p *Plugin) Validate(c *v1alpha1.ChainCluster) []domain.Diagnostic {
	diags := make([]domain.Diagnostic, 0)

	if strings.TrimSpace(c.Spec.Plugin) != PluginName {
		diags = append(diags, domain.Error(
			"spec.plugin",
			"evm-family plugin selected with mismatched plugin name",
			"set spec.plugin to evm-family",
		))
	}

	if strings.TrimSpace(c.Spec.Family) == "" {
		diags = append(diags, domain.Error(
			"spec.family",
			"family must be set",
			"set family to evm (or ethereum alias)",
		))
	} else if normalizeFamily(c.Spec.Family) != FamilyName {
		diags = append(diags, domain.Warning(
			"spec.family",
			"plugin is optimized for family evm",
			"prefer family: evm",
		))
	}

	for i, pool := range c.Spec.NodePools {
		poolPath := fmt.Sprintf("spec.nodePools[%d].template", i)
		workloadIndex, workload := findEVMWorkload(pool.Template.Workloads)
		hasTypedClient := false
		if pool.Template.PluginConfig.EVM != nil && strings.TrimSpace(pool.Template.PluginConfig.EVM.Client) != "" {
			hasTypedClient = true
		}
		if !hasTypedClient {
			for _, w := range pool.Template.Workloads {
				if w.PluginConfig.EVM != nil && strings.TrimSpace(w.PluginConfig.EVM.Client) != "" {
					hasTypedClient = true
					break
				}
			}
		}
		if workloadIndex < 0 && !hasTypedClient {
			diags = append(diags, domain.Error(
				poolPath+".workloads",
				"evm-family requires at least one EVM workload per node template",
				"define one workload with geth/erigon/nethermind/besu/reth command or image",
			))
			continue
		}

		if workload == nil {
			continue
		}
		if !hasPort(workload.Ports, 30303) {
			diags = append(diags, domain.Warning(
				fmt.Sprintf("%s.workloads[%d].ports", poolPath, workloadIndex),
				"p2p port 30303 not declared",
				"declare containerPort 30303 for peer connectivity",
			))
		}
		if !hasPort(workload.Ports, 8545) {
			diags = append(diags, domain.Warning(
				fmt.Sprintf("%s.workloads[%d].ports", poolPath, workloadIndex),
				"http rpc port 8545 not declared",
				"declare containerPort 8545 for JSON-RPC",
			))
		}
	}

	return diags
}

// Normalize infers default family/profile and canonical config formatting.
func (p *Plugin) Normalize(c *v1alpha1.ChainCluster) error {
	if normalized := normalizeFamily(c.Spec.Family); normalized != "" {
		c.Spec.Family = normalized
	} else {
		c.Spec.Family = FamilyName
	}
	if strings.TrimSpace(c.Spec.Profile) == "" {
		c.Spec.Profile = "ethereum-rpc"
	}
	normalizeEVMConfig(c.Spec.PluginConfig.EVM)
	for i := range c.Spec.NodePools {
		pool := &c.Spec.NodePools[i]
		normalizeEVMConfig(pool.Template.PluginConfig.EVM)
		for j := range pool.Template.Workloads {
			normalizeEVMConfig(pool.Template.Workloads[j].PluginConfig.EVM)
		}
	}
	return nil
}

// Build renders EVM node env artifacts with deterministic ordering.
func (p *Plugin) Build(ctx context.Context, c *v1alpha1.ChainCluster) (chain.Output, error) {
	_ = ctx

	artifactsByPath := make(map[string]string)
	nodes := spec.ExpandNodes(c)
	for _, n := range nodes {
		workloadIndex, workload := findEVMWorkload(n.Spec.Workloads)
		var workloadCfg *v1alpha1.EVMConfig
		if workloadIndex >= 0 {
			workloadCfg = n.Spec.Workloads[workloadIndex].PluginConfig.EVM
		}

		resolved := resolveEVMConfig(
			workload,
			c.Spec.PluginConfig.EVM,
			n.Spec.PluginConfig.EVM,
			workloadCfg,
		)
		addArtifact(artifactsByPath, filepath.Join("nodes", n.Name, "config", "evm.env"), renderEnvFile(resolved))
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

type resolvedEVMConfig struct {
	Client         string
	Network        string
	ChainID        int64
	SyncMode       string
	HTTPEnabled    bool
	WSEnabled      bool
	AuthRPCEnabled bool
	MetricsEnabled bool
	P2PPort        int
	HTTPPort       int
	WSPort         int
	AuthRPCPort    int
	MetricsPort    int
	Bootnodes      string
}

func resolveEVMConfig(
	workload *v1alpha1.WorkloadSpec,
	clusterCfg *v1alpha1.EVMConfig,
	nodeCfg *v1alpha1.EVMConfig,
	workloadCfg *v1alpha1.EVMConfig,
) resolvedEVMConfig {
	merged := mergeEVMConfig(clusterCfg, nodeCfg, workloadCfg)

	client := inferClientFromConfigOrWorkload(merged.Client, workload)
	network := "mainnet"
	if merged.Network != "" {
		network = merged.Network
	}

	p2pPort := 30303
	httpPort := 8545
	wsPort := 8546
	authRPCPort := 8551
	metricsPort := 6060
	if workload != nil {
		p2pPort = firstPortByNameOrDefault(workload.Ports, []string{"p2p", "discovery", "eth-p2p"}, p2pPort)
		httpPort = firstPortByNameOrDefault(workload.Ports, []string{"rpc", "http", "json-rpc"}, httpPort)
		wsPort = firstPortByNameOrDefault(workload.Ports, []string{"ws", "websocket"}, wsPort)
		authRPCPort = firstPortByNameOrDefault(workload.Ports, []string{"authrpc", "engine"}, authRPCPort)
		metricsPort = firstPortByNameOrDefault(workload.Ports, []string{"metrics", "prometheus"}, metricsPort)
	}

	cfg := resolvedEVMConfig{
		Client:         client,
		Network:        network,
		ChainID:        chainIDForNetwork(network),
		SyncMode:       "snap",
		HTTPEnabled:    true,
		WSEnabled:      false,
		AuthRPCEnabled: true,
		MetricsEnabled: false,
		P2PPort:        p2pPort,
		HTTPPort:       httpPort,
		WSPort:         wsPort,
		AuthRPCPort:    authRPCPort,
		MetricsPort:    metricsPort,
		Bootnodes:      strings.Join(merged.Bootnodes, ","),
	}

	if merged.ChainID > 0 {
		cfg.ChainID = merged.ChainID
	}
	if merged.SyncMode != "" {
		cfg.SyncMode = merged.SyncMode
	}
	if merged.P2PPort > 0 {
		cfg.P2PPort = merged.P2PPort
	}
	if merged.HTTPPort > 0 {
		cfg.HTTPPort = merged.HTTPPort
	}
	if merged.WSPort > 0 {
		cfg.WSPort = merged.WSPort
	}
	if merged.AuthRPCPort > 0 {
		cfg.AuthRPCPort = merged.AuthRPCPort
	}
	if merged.MetricsPort > 0 {
		cfg.MetricsPort = merged.MetricsPort
	}
	if merged.HTTPEnabled != nil {
		cfg.HTTPEnabled = *merged.HTTPEnabled
	}
	if merged.WSEnabled != nil {
		cfg.WSEnabled = *merged.WSEnabled
	}
	if merged.AuthRPCEnabled != nil {
		cfg.AuthRPCEnabled = *merged.AuthRPCEnabled
	}
	if merged.MetricsEnabled != nil {
		cfg.MetricsEnabled = *merged.MetricsEnabled
	}

	return cfg
}

func renderEnvFile(cfg resolvedEVMConfig) string {
	var b strings.Builder
	b.WriteString("# Generated by bgorch evm-family plugin\n")
	b.WriteString("EVM_CLIENT=" + cfg.Client + "\n")
	b.WriteString("EVM_NETWORK=" + cfg.Network + "\n")
	b.WriteString("EVM_CHAIN_ID=" + strconv.FormatInt(cfg.ChainID, 10) + "\n")
	b.WriteString("EVM_SYNC_MODE=" + cfg.SyncMode + "\n")
	b.WriteString("EVM_HTTP_ENABLED=" + strconv.FormatBool(cfg.HTTPEnabled) + "\n")
	b.WriteString("EVM_WS_ENABLED=" + strconv.FormatBool(cfg.WSEnabled) + "\n")
	b.WriteString("EVM_AUTHRPC_ENABLED=" + strconv.FormatBool(cfg.AuthRPCEnabled) + "\n")
	b.WriteString("EVM_METRICS_ENABLED=" + strconv.FormatBool(cfg.MetricsEnabled) + "\n")
	b.WriteString("EVM_P2P_PORT=" + strconv.Itoa(cfg.P2PPort) + "\n")
	b.WriteString("EVM_HTTP_PORT=" + strconv.Itoa(cfg.HTTPPort) + "\n")
	b.WriteString("EVM_WS_PORT=" + strconv.Itoa(cfg.WSPort) + "\n")
	b.WriteString("EVM_AUTHRPC_PORT=" + strconv.Itoa(cfg.AuthRPCPort) + "\n")
	b.WriteString("EVM_METRICS_PORT=" + strconv.Itoa(cfg.MetricsPort) + "\n")
	b.WriteString("EVM_BOOTNODES=" + cfg.Bootnodes + "\n")
	return b.String()
}

func mergeEVMConfig(configs ...*v1alpha1.EVMConfig) v1alpha1.EVMConfig {
	var out v1alpha1.EVMConfig
	for _, cfg := range configs {
		if cfg == nil {
			continue
		}
		if cfg.Client != "" {
			out.Client = cfg.Client
		}
		if cfg.Network != "" {
			out.Network = cfg.Network
		}
		if cfg.ChainID > 0 {
			out.ChainID = cfg.ChainID
		}
		if cfg.SyncMode != "" {
			out.SyncMode = cfg.SyncMode
		}
		if cfg.P2PPort > 0 {
			out.P2PPort = cfg.P2PPort
		}
		if cfg.HTTPPort > 0 {
			out.HTTPPort = cfg.HTTPPort
		}
		if cfg.WSPort > 0 {
			out.WSPort = cfg.WSPort
		}
		if cfg.AuthRPCPort > 0 {
			out.AuthRPCPort = cfg.AuthRPCPort
		}
		if cfg.MetricsPort > 0 {
			out.MetricsPort = cfg.MetricsPort
		}
		if cfg.HTTPEnabled != nil {
			v := *cfg.HTTPEnabled
			out.HTTPEnabled = &v
		}
		if cfg.WSEnabled != nil {
			v := *cfg.WSEnabled
			out.WSEnabled = &v
		}
		if cfg.AuthRPCEnabled != nil {
			v := *cfg.AuthRPCEnabled
			out.AuthRPCEnabled = &v
		}
		if cfg.MetricsEnabled != nil {
			v := *cfg.MetricsEnabled
			out.MetricsEnabled = &v
		}
		if len(cfg.Bootnodes) > 0 {
			out.Bootnodes = filterNonEmpty(cfg.Bootnodes)
		}
	}
	return out
}

func normalizeEVMConfig(cfg *v1alpha1.EVMConfig) {
	if cfg == nil {
		return
	}
	cfg.Client = normalizeClient(cfg.Client)
	cfg.Network = normalizeNetwork(cfg.Network)
	cfg.SyncMode = normalizeSyncMode(cfg.SyncMode)
	cfg.Bootnodes = filterNonEmpty(cfg.Bootnodes)
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
	return "geth"
}

func inferClient(workload v1alpha1.WorkloadSpec) string {
	parts := []string{workload.Name, workload.Image, workload.Binary}
	parts = append(parts, workload.Command...)
	parts = append(parts, workload.Args...)

	for _, part := range parts {
		value := strings.ToLower(part)
		switch {
		case strings.Contains(value, "nethermind"):
			return "nethermind"
		case strings.Contains(value, "erigon"):
			return "erigon"
		case strings.Contains(value, "besu"):
			return "besu"
		case strings.Contains(value, "reth"):
			return "reth"
		case strings.Contains(value, "geth"), strings.Contains(value, "go-ethereum"), strings.Contains(value, "ethereum"):
			return "geth"
		}
	}
	return ""
}

func findEVMWorkload(workloads []v1alpha1.WorkloadSpec) (int, *v1alpha1.WorkloadSpec) {
	for i := range workloads {
		if isEVMWorkload(workloads[i]) {
			return i, &workloads[i]
		}
	}
	return -1, nil
}

func isEVMWorkload(w v1alpha1.WorkloadSpec) bool {
	if inferClient(w) != "" {
		return true
	}
	parts := []string{w.Name, w.Image, w.Binary}
	parts = append(parts, w.Command...)
	parts = append(parts, w.Args...)
	for _, part := range parts {
		value := strings.ToLower(part)
		if strings.Contains(value, "evm") || strings.Contains(value, "execution") {
			return true
		}
	}
	return false
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

func chainIDForNetwork(network string) int64 {
	switch normalizeNetwork(network) {
	case "sepolia":
		return 11155111
	case "holesky":
		return 17000
	case "devnet":
		return 1337
	case "local":
		return 31337
	default:
		return 1
	}
}

func normalizeFamily(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "evm", "ethereum":
		return FamilyName
	default:
		return ""
	}
}

func normalizeClient(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "go-ethereum":
		return "geth"
	case "geth", "erigon", "nethermind", "besu", "reth":
		return value
	default:
		return value
	}
}

func normalizeNetwork(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "mainnet", "sepolia", "holesky", "devnet", "local":
		return value
	default:
		return value
	}
}

func normalizeSyncMode(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "full", "snap", "archive", "light":
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
