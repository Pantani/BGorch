package cosmos

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
	// PluginName is the registry identifier for the cosmos plugin.
	PluginName = "cosmos-family"
	// FamilyName is the normalized chain family handled by this plugin.
	FamilyName = "cosmos"
)

var _ chain.Plugin = (*Plugin)(nil)

// Plugin implements chain.Plugin for Cosmos-SDK oriented defaults and artifacts.
type Plugin struct{}

// New returns a cosmos-family plugin instance.
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

// Validate checks cosmos-family assumptions around workloads and common ports.
func (p *Plugin) Validate(c *v1alpha1.ChainCluster) []domain.Diagnostic {
	diags := make([]domain.Diagnostic, 0)

	if strings.TrimSpace(c.Spec.Plugin) != PluginName {
		diags = append(diags, domain.Error(
			"spec.plugin",
			"cosmos-family plugin selected with mismatched plugin name",
			"set spec.plugin to cosmos-family",
		))
	}

	if strings.TrimSpace(c.Spec.Family) == "" {
		diags = append(diags, domain.Error(
			"spec.family",
			"family must be set",
			"set family to cosmos",
		))
	} else if normalizeFamily(c.Spec.Family) != FamilyName {
		diags = append(diags, domain.Warning(
			"spec.family",
			"plugin is optimized for family cosmos",
			"prefer family: cosmos",
		))
	}

	for i, pool := range c.Spec.NodePools {
		poolPath := fmt.Sprintf("spec.nodePools[%d].template", i)
		workloadIndex, workload := findCosmosWorkload(pool.Template.Workloads)
		hasTypedClient := false
		if pool.Template.PluginConfig.Cosmos != nil && strings.TrimSpace(pool.Template.PluginConfig.Cosmos.Client) != "" {
			hasTypedClient = true
		}
		if !hasTypedClient {
			for _, w := range pool.Template.Workloads {
				if w.PluginConfig.Cosmos != nil && strings.TrimSpace(w.PluginConfig.Cosmos.Client) != "" {
					hasTypedClient = true
					break
				}
			}
		}
		if workloadIndex < 0 && !hasTypedClient {
			diags = append(diags, domain.Error(
				poolPath+".workloads",
				"cosmos-family requires at least one Cosmos workload per node template",
				"define one workload with gaiad/osmosisd/simd/cometbft command or image",
			))
			continue
		}

		if workload == nil {
			continue
		}
		if !hasPort(workload.Ports, 26656) {
			diags = append(diags, domain.Warning(
				fmt.Sprintf("%s.workloads[%d].ports", poolPath, workloadIndex),
				"p2p port 26656 not declared",
				"declare containerPort 26656 for peer connectivity",
			))
		}
		if !hasPort(workload.Ports, 26657) {
			diags = append(diags, domain.Warning(
				fmt.Sprintf("%s.workloads[%d].ports", poolPath, workloadIndex),
				"rpc port 26657 not declared",
				"declare containerPort 26657 for RPC and health operations",
			))
		}
	}

	return diags
}

// Normalize infers default family/profile and canonical config formatting.
func (p *Plugin) Normalize(c *v1alpha1.ChainCluster) error {
	c.Spec.Family = FamilyName
	if strings.TrimSpace(c.Spec.Profile) == "" {
		c.Spec.Profile = "cosmos-validator"
	}
	normalizeCosmosConfig(c.Spec.PluginConfig.Cosmos)
	for i := range c.Spec.NodePools {
		pool := &c.Spec.NodePools[i]
		normalizeCosmosConfig(pool.Template.PluginConfig.Cosmos)
		for j := range pool.Template.Workloads {
			normalizeCosmosConfig(pool.Template.Workloads[j].PluginConfig.Cosmos)
		}
	}
	return nil
}

// Build renders Cosmos config artifacts with deterministic ordering.
func (p *Plugin) Build(ctx context.Context, c *v1alpha1.ChainCluster) (chain.Output, error) {
	_ = ctx

	artifactsByPath := make(map[string]string)
	nodes := spec.ExpandNodes(c)
	for _, n := range nodes {
		workloadIndex, workload := findCosmosWorkload(n.Spec.Workloads)
		var workloadCfg *v1alpha1.CosmosConfig
		if workloadIndex >= 0 {
			workloadCfg = n.Spec.Workloads[workloadIndex].PluginConfig.Cosmos
		}

		resolved := resolveCosmosConfig(
			c.Metadata.Name,
			n.Name,
			workload,
			c.Spec.PluginConfig.Cosmos,
			n.Spec.PluginConfig.Cosmos,
			workloadCfg,
		)
		addArtifact(artifactsByPath, filepath.Join("nodes", n.Name, "config", "cosmos.env"), renderEnvFile(resolved))
		addArtifact(artifactsByPath, filepath.Join("nodes", n.Name, "config", "config.toml"), renderConfigToml(resolved))
		addArtifact(artifactsByPath, filepath.Join("nodes", n.Name, "config", "app.toml"), renderAppToml(resolved))
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

type resolvedCosmosConfig struct {
	Client           string
	ChainID          string
	Moniker          string
	DaemonBinary     string
	HomeDir          string
	P2PPort          int
	RPCPort          int
	APIEnabled       bool
	APIPort          int
	GRPCEnabled      bool
	GRPCPort         int
	Pruning          string
	MinimumGasPrices string
	Seeds            string
	PersistentPeers  string
}

func resolveCosmosConfig(
	clusterName string,
	nodeName string,
	workload *v1alpha1.WorkloadSpec,
	clusterCfg *v1alpha1.CosmosConfig,
	nodeCfg *v1alpha1.CosmosConfig,
	workloadCfg *v1alpha1.CosmosConfig,
) resolvedCosmosConfig {
	merged := mergeCosmosConfig(clusterCfg, nodeCfg, workloadCfg)
	client := "cosmos-sdk"
	if merged.Client != "" {
		client = merged.Client
	}
	daemonBinary := inferBinaryFromConfigOrWorkload(merged.DaemonBinary, workload)
	p2pPort := 26656
	rpcPort := 26657
	if workload != nil {
		p2pPort = firstPortByNameOrDefault(workload.Ports, []string{"p2p"}, p2pPort)
		rpcPort = firstPortByNameOrDefault(workload.Ports, []string{"rpc"}, rpcPort)
	}

	cfg := resolvedCosmosConfig{
		Client:           client,
		ChainID:          fmt.Sprintf("%s-localnet", sanitizeIdentifier(clusterName)),
		Moniker:          sanitizeIdentifier(nodeName),
		DaemonBinary:     daemonBinary,
		HomeDir:          "/var/lib/cosmos",
		P2PPort:          p2pPort,
		RPCPort:          rpcPort,
		APIEnabled:       true,
		APIPort:          1317,
		GRPCEnabled:      true,
		GRPCPort:         9090,
		Pruning:          "default",
		MinimumGasPrices: "0stake",
		Seeds:            strings.Join(merged.Seeds, ","),
		PersistentPeers:  strings.Join(merged.PersistentPeers, ","),
	}

	if merged.ChainID != "" {
		cfg.ChainID = merged.ChainID
	}
	if merged.Moniker != "" {
		cfg.Moniker = sanitizeIdentifier(merged.Moniker)
	}
	if merged.HomeDir != "" {
		cfg.HomeDir = merged.HomeDir
	}
	if merged.P2PPort > 0 {
		cfg.P2PPort = merged.P2PPort
	}
	if merged.RPCPort > 0 {
		cfg.RPCPort = merged.RPCPort
	}
	if merged.APIPort > 0 {
		cfg.APIPort = merged.APIPort
	}
	if merged.GRPCPort > 0 {
		cfg.GRPCPort = merged.GRPCPort
	}
	if merged.APIEnabled != nil {
		cfg.APIEnabled = *merged.APIEnabled
	}
	if merged.GRPCEnabled != nil {
		cfg.GRPCEnabled = *merged.GRPCEnabled
	}
	if merged.Pruning != "" {
		cfg.Pruning = merged.Pruning
	}
	if merged.MinimumGasPrices != "" {
		cfg.MinimumGasPrices = merged.MinimumGasPrices
	}

	return cfg
}

func renderEnvFile(cfg resolvedCosmosConfig) string {
	var b strings.Builder
	b.WriteString("# Generated by bgorch cosmos-family plugin\n")
	b.WriteString("COSMOS_CLIENT=" + cfg.Client + "\n")
	b.WriteString("COSMOS_CHAIN_ID=" + cfg.ChainID + "\n")
	b.WriteString("COSMOS_MONIKER=" + cfg.Moniker + "\n")
	b.WriteString("COSMOS_DAEMON_BINARY=" + cfg.DaemonBinary + "\n")
	b.WriteString("COSMOS_HOME=" + cfg.HomeDir + "\n")
	b.WriteString("COSMOS_P2P_PORT=" + strconv.Itoa(cfg.P2PPort) + "\n")
	b.WriteString("COSMOS_RPC_PORT=" + strconv.Itoa(cfg.RPCPort) + "\n")
	b.WriteString("COSMOS_API_ENABLED=" + strconv.FormatBool(cfg.APIEnabled) + "\n")
	b.WriteString("COSMOS_API_PORT=" + strconv.Itoa(cfg.APIPort) + "\n")
	b.WriteString("COSMOS_GRPC_ENABLED=" + strconv.FormatBool(cfg.GRPCEnabled) + "\n")
	b.WriteString("COSMOS_GRPC_PORT=" + strconv.Itoa(cfg.GRPCPort) + "\n")
	b.WriteString("COSMOS_PRUNING=" + cfg.Pruning + "\n")
	b.WriteString("COSMOS_MINIMUM_GAS_PRICES=" + cfg.MinimumGasPrices + "\n")
	b.WriteString("COSMOS_SEEDS=" + cfg.Seeds + "\n")
	b.WriteString("COSMOS_PERSISTENT_PEERS=" + cfg.PersistentPeers + "\n")
	return b.String()
}

func renderConfigToml(cfg resolvedCosmosConfig) string {
	var b strings.Builder
	b.WriteString("# Generated by bgorch cosmos-family plugin\n")
	b.WriteString("moniker = " + strconv.Quote(cfg.Moniker) + "\n")
	b.WriteString("proxy_app = \"tcp://127.0.0.1:26658\"\n\n")
	b.WriteString("[rpc]\n")
	b.WriteString("laddr = " + strconv.Quote(fmt.Sprintf("tcp://0.0.0.0:%d", cfg.RPCPort)) + "\n\n")
	b.WriteString("[p2p]\n")
	b.WriteString("laddr = " + strconv.Quote(fmt.Sprintf("tcp://0.0.0.0:%d", cfg.P2PPort)) + "\n")
	b.WriteString("seeds = " + strconv.Quote(cfg.Seeds) + "\n")
	b.WriteString("persistent_peers = " + strconv.Quote(cfg.PersistentPeers) + "\n")
	return b.String()
}

func renderAppToml(cfg resolvedCosmosConfig) string {
	var b strings.Builder
	b.WriteString("# Generated by bgorch cosmos-family plugin\n")
	b.WriteString("minimum-gas-prices = " + strconv.Quote(cfg.MinimumGasPrices) + "\n")
	b.WriteString("pruning = " + strconv.Quote(cfg.Pruning) + "\n\n")
	b.WriteString("[api]\n")
	b.WriteString("enable = " + strconv.FormatBool(cfg.APIEnabled) + "\n")
	b.WriteString("address = " + strconv.Quote(fmt.Sprintf("tcp://0.0.0.0:%d", cfg.APIPort)) + "\n\n")
	b.WriteString("[grpc]\n")
	b.WriteString("enable = " + strconv.FormatBool(cfg.GRPCEnabled) + "\n")
	b.WriteString("address = " + strconv.Quote(fmt.Sprintf("0.0.0.0:%d", cfg.GRPCPort)) + "\n")
	return b.String()
}

func mergeCosmosConfig(configs ...*v1alpha1.CosmosConfig) v1alpha1.CosmosConfig {
	var out v1alpha1.CosmosConfig
	for _, cfg := range configs {
		if cfg == nil {
			continue
		}
		if cfg.Client != "" {
			out.Client = cfg.Client
		}
		if cfg.ChainID != "" {
			out.ChainID = cfg.ChainID
		}
		if cfg.Moniker != "" {
			out.Moniker = cfg.Moniker
		}
		if cfg.DaemonBinary != "" {
			out.DaemonBinary = cfg.DaemonBinary
		}
		if cfg.HomeDir != "" {
			out.HomeDir = cfg.HomeDir
		}
		if cfg.P2PPort > 0 {
			out.P2PPort = cfg.P2PPort
		}
		if cfg.RPCPort > 0 {
			out.RPCPort = cfg.RPCPort
		}
		if cfg.APIPort > 0 {
			out.APIPort = cfg.APIPort
		}
		if cfg.GRPCPort > 0 {
			out.GRPCPort = cfg.GRPCPort
		}
		if cfg.APIEnabled != nil {
			v := *cfg.APIEnabled
			out.APIEnabled = &v
		}
		if cfg.GRPCEnabled != nil {
			v := *cfg.GRPCEnabled
			out.GRPCEnabled = &v
		}
		if cfg.Pruning != "" {
			out.Pruning = cfg.Pruning
		}
		if cfg.MinimumGasPrices != "" {
			out.MinimumGasPrices = cfg.MinimumGasPrices
		}
		if len(cfg.Seeds) > 0 {
			out.Seeds = filterNonEmpty(cfg.Seeds)
		}
		if len(cfg.PersistentPeers) > 0 {
			out.PersistentPeers = filterNonEmpty(cfg.PersistentPeers)
		}
	}
	return out
}

func normalizeCosmosConfig(cfg *v1alpha1.CosmosConfig) {
	if cfg == nil {
		return
	}
	cfg.Client = strings.ToLower(strings.TrimSpace(cfg.Client))
	cfg.ChainID = strings.TrimSpace(cfg.ChainID)
	cfg.Moniker = strings.TrimSpace(cfg.Moniker)
	cfg.DaemonBinary = strings.TrimSpace(cfg.DaemonBinary)
	cfg.HomeDir = strings.TrimSpace(cfg.HomeDir)
	cfg.Pruning = strings.ToLower(strings.TrimSpace(cfg.Pruning))
	cfg.MinimumGasPrices = strings.TrimSpace(cfg.MinimumGasPrices)
	cfg.Seeds = filterNonEmpty(cfg.Seeds)
	cfg.PersistentPeers = filterNonEmpty(cfg.PersistentPeers)
}

func inferBinaryFromConfigOrWorkload(configured string, workload *v1alpha1.WorkloadSpec) string {
	configured = strings.TrimSpace(configured)
	if configured != "" {
		return configured
	}
	if workload == nil {
		return "gaiad"
	}
	if len(workload.Command) > 0 && strings.TrimSpace(workload.Command[0]) != "" {
		return strings.TrimSpace(workload.Command[0])
	}
	if strings.TrimSpace(workload.Binary) != "" {
		return strings.TrimSpace(workload.Binary)
	}
	client := inferClient(*workload)
	switch client {
	case "osmosisd":
		return "osmosisd"
	case "simd":
		return "simd"
	default:
		return "gaiad"
	}
}

func inferClient(workload v1alpha1.WorkloadSpec) string {
	parts := []string{workload.Name, workload.Image, workload.Binary}
	parts = append(parts, workload.Command...)
	parts = append(parts, workload.Args...)
	for _, part := range parts {
		value := strings.ToLower(part)
		switch {
		case strings.Contains(value, "osmosisd"):
			return "osmosisd"
		case strings.Contains(value, "simd"):
			return "simd"
		case strings.Contains(value, "gaiad"):
			return "gaiad"
		case strings.Contains(value, "cosmos"), strings.Contains(value, "cometbft"), strings.Contains(value, "tendermint"):
			return "cosmos-sdk"
		}
	}
	return ""
}

func findCosmosWorkload(workloads []v1alpha1.WorkloadSpec) (int, *v1alpha1.WorkloadSpec) {
	for i := range workloads {
		if isCosmosWorkload(workloads[i]) {
			return i, &workloads[i]
		}
	}
	return -1, nil
}

func isCosmosWorkload(w v1alpha1.WorkloadSpec) bool {
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

func sanitizeIdentifier(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	repl := strings.NewReplacer(" ", "-", "/", "-", "_", "-", ".", "-", ":", "-")
	v = repl.Replace(v)
	parts := strings.FieldsFunc(v, func(r rune) bool {
		return r != '-' && (r < 'a' || r > 'z') && (r < '0' || r > '9')
	})
	v = strings.Join(parts, "-")
	v = strings.Trim(v, "-")
	if v == "" {
		return "node"
	}
	return v
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
