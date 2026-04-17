package bitcoin

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
	// PluginName is the registry identifier for the bitcoin plugin.
	PluginName = "bitcoin-family"
	// FamilyName is the normalized chain family handled by this plugin.
	FamilyName = "bitcoin"
)

var _ chain.Plugin = (*Plugin)(nil)

// Plugin implements chain.Plugin for Bitcoin defaults and artifacts.
type Plugin struct{}

// New returns a bitcoin-family plugin instance.
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

// Validate checks bitcoin-family assumptions around workloads and common ports.
func (p *Plugin) Validate(c *v1alpha1.ChainCluster) []domain.Diagnostic {
	diags := make([]domain.Diagnostic, 0)

	if strings.TrimSpace(c.Spec.Plugin) != PluginName {
		diags = append(diags, domain.Error(
			"spec.plugin",
			"bitcoin-family plugin selected with mismatched plugin name",
			"set spec.plugin to bitcoin-family",
		))
	}

	if strings.TrimSpace(c.Spec.Family) == "" {
		diags = append(diags, domain.Error(
			"spec.family",
			"family must be set",
			"set family to bitcoin (or btc alias)",
		))
	} else if normalizeFamily(c.Spec.Family) != FamilyName {
		diags = append(diags, domain.Warning(
			"spec.family",
			"plugin is optimized for family bitcoin",
			"prefer family: bitcoin",
		))
	}

	for i, pool := range c.Spec.NodePools {
		poolPath := fmt.Sprintf("spec.nodePools[%d].template", i)
		workloadIndex, workload := findBitcoinWorkload(pool.Template.Workloads)
		hasTypedClient := false
		if pool.Template.PluginConfig.Bitcoin != nil && strings.TrimSpace(pool.Template.PluginConfig.Bitcoin.Client) != "" {
			hasTypedClient = true
		}
		if !hasTypedClient {
			for _, w := range pool.Template.Workloads {
				if w.PluginConfig.Bitcoin != nil && strings.TrimSpace(w.PluginConfig.Bitcoin.Client) != "" {
					hasTypedClient = true
					break
				}
			}
		}
		if workloadIndex < 0 && !hasTypedClient {
			diags = append(diags, domain.Error(
				poolPath+".workloads",
				"bitcoin-family requires at least one Bitcoin workload per node template",
				"define one workload with bitcoind/bitcoin-core/btcd command or image",
			))
			continue
		}

		if workload == nil {
			continue
		}
		if !hasPort(workload.Ports, 8333) {
			diags = append(diags, domain.Warning(
				fmt.Sprintf("%s.workloads[%d].ports", poolPath, workloadIndex),
				"p2p port 8333 not declared",
				"declare containerPort 8333 for peer connectivity",
			))
		}
		if !hasPort(workload.Ports, 8332) {
			diags = append(diags, domain.Warning(
				fmt.Sprintf("%s.workloads[%d].ports", poolPath, workloadIndex),
				"rpc port 8332 not declared",
				"declare containerPort 8332 for JSON-RPC",
			))
		}
	}

	return diags
}

// Normalize infers default family/profile and canonical config formatting.
func (p *Plugin) Normalize(c *v1alpha1.ChainCluster) error {
	c.Spec.Family = FamilyName
	if strings.TrimSpace(c.Spec.Profile) == "" {
		c.Spec.Profile = "bitcoin-node"
	}
	normalizeBitcoinConfig(c.Spec.PluginConfig.Bitcoin)
	for i := range c.Spec.NodePools {
		pool := &c.Spec.NodePools[i]
		normalizeBitcoinConfig(pool.Template.PluginConfig.Bitcoin)
		for j := range pool.Template.Workloads {
			normalizeBitcoinConfig(pool.Template.Workloads[j].PluginConfig.Bitcoin)
		}
	}
	return nil
}

// Build renders bitcoin.conf artifacts with deterministic ordering.
func (p *Plugin) Build(ctx context.Context, c *v1alpha1.ChainCluster) (chain.Output, error) {
	_ = ctx

	artifactsByPath := make(map[string]string)
	nodes := spec.ExpandNodes(c)
	for _, n := range nodes {
		workloadIndex, workload := findBitcoinWorkload(n.Spec.Workloads)
		var workloadCfg *v1alpha1.BitcoinConfig
		if workloadIndex >= 0 {
			workloadCfg = n.Spec.Workloads[workloadIndex].PluginConfig.Bitcoin
		}

		resolved := resolveBitcoinConfig(
			workload,
			c.Spec.PluginConfig.Bitcoin,
			n.Spec.PluginConfig.Bitcoin,
			workloadCfg,
		)
		addArtifact(artifactsByPath, filepath.Join("nodes", n.Name, "config", "bitcoin.conf"), renderBitcoinConf(resolved))
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

type resolvedBitcoinConfig struct {
	Client       string
	Network      string
	RPCPort      int
	P2PPort      int
	ZMQBlockAddr string
	ZMQTxAddr    string
	TxIndex      bool
	PruneMB      int
	ExtraArgs    []string
}

func resolveBitcoinConfig(
	workload *v1alpha1.WorkloadSpec,
	clusterCfg *v1alpha1.BitcoinConfig,
	nodeCfg *v1alpha1.BitcoinConfig,
	workloadCfg *v1alpha1.BitcoinConfig,
) resolvedBitcoinConfig {
	merged := mergeBitcoinConfig(clusterCfg, nodeCfg, workloadCfg)
	client := inferClientFromConfigOrWorkload(merged.Client, workload)
	network := "mainnet"
	if merged.Network != "" {
		network = merged.Network
	}
	rpcDefault, p2pDefault := defaultPortsForNetwork(network)
	rpcPort := rpcDefault
	p2pPort := p2pDefault
	if workload != nil {
		rpcPort = firstPortByNameOrDefault(workload.Ports, []string{"rpc", "json-rpc"}, rpcPort)
		p2pPort = firstPortByNameOrDefault(workload.Ports, []string{"p2p"}, p2pPort)
	}

	cfg := resolvedBitcoinConfig{
		Client:    client,
		Network:   network,
		RPCPort:   rpcPort,
		P2PPort:   p2pPort,
		TxIndex:   true,
		PruneMB:   0,
		ExtraArgs: append([]string{}, merged.ExtraArgs...),
	}

	if merged.RPCPort > 0 {
		cfg.RPCPort = merged.RPCPort
	}
	if merged.P2PPort > 0 {
		cfg.P2PPort = merged.P2PPort
	}
	if merged.ZMQBlockAddr != "" {
		cfg.ZMQBlockAddr = merged.ZMQBlockAddr
	}
	if merged.ZMQTxAddr != "" {
		cfg.ZMQTxAddr = merged.ZMQTxAddr
	}
	if merged.TxIndex != nil {
		cfg.TxIndex = *merged.TxIndex
	}
	if merged.PruneMB > 0 {
		cfg.PruneMB = merged.PruneMB
	}

	return cfg
}

func renderBitcoinConf(cfg resolvedBitcoinConfig) string {
	var b strings.Builder
	b.WriteString("# Generated by bgorch bitcoin-family plugin\n")
	switch cfg.Network {
	case "testnet":
		b.WriteString("testnet=1\n")
	case "signet":
		b.WriteString("signet=1\n")
	case "regtest":
		b.WriteString("regtest=1\n")
	}
	b.WriteString("server=1\n")
	b.WriteString("listen=1\n")
	b.WriteString("txindex=" + boolAsInt(cfg.TxIndex) + "\n")
	b.WriteString("rpcbind=0.0.0.0\n")
	b.WriteString("rpcallowip=0.0.0.0/0\n")
	b.WriteString("rpcport=" + strconv.Itoa(cfg.RPCPort) + "\n")
	b.WriteString("port=" + strconv.Itoa(cfg.P2PPort) + "\n")
	if cfg.PruneMB > 0 {
		b.WriteString("prune=" + strconv.Itoa(cfg.PruneMB) + "\n")
	}
	if cfg.ZMQBlockAddr != "" {
		b.WriteString("zmqpubrawblock=" + cfg.ZMQBlockAddr + "\n")
	}
	if cfg.ZMQTxAddr != "" {
		b.WriteString("zmqpubrawtx=" + cfg.ZMQTxAddr + "\n")
	}
	for _, arg := range cfg.ExtraArgs {
		b.WriteString("extraarg=" + arg + "\n")
	}
	return b.String()
}

func mergeBitcoinConfig(configs ...*v1alpha1.BitcoinConfig) v1alpha1.BitcoinConfig {
	var out v1alpha1.BitcoinConfig
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
		if cfg.RPCPort > 0 {
			out.RPCPort = cfg.RPCPort
		}
		if cfg.P2PPort > 0 {
			out.P2PPort = cfg.P2PPort
		}
		if cfg.ZMQBlockAddr != "" {
			out.ZMQBlockAddr = cfg.ZMQBlockAddr
		}
		if cfg.ZMQTxAddr != "" {
			out.ZMQTxAddr = cfg.ZMQTxAddr
		}
		if cfg.TxIndex != nil {
			v := *cfg.TxIndex
			out.TxIndex = &v
		}
		if cfg.PruneMB > 0 {
			out.PruneMB = cfg.PruneMB
		}
		if len(cfg.ExtraArgs) > 0 {
			out.ExtraArgs = filterNonEmpty(cfg.ExtraArgs)
		}
	}
	return out
}

func normalizeBitcoinConfig(cfg *v1alpha1.BitcoinConfig) {
	if cfg == nil {
		return
	}
	cfg.Client = normalizeClient(cfg.Client)
	cfg.Network = normalizeNetwork(cfg.Network)
	cfg.ZMQBlockAddr = strings.TrimSpace(cfg.ZMQBlockAddr)
	cfg.ZMQTxAddr = strings.TrimSpace(cfg.ZMQTxAddr)
	cfg.ExtraArgs = filterNonEmpty(cfg.ExtraArgs)
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
	return "bitcoin-core"
}

func inferClient(workload v1alpha1.WorkloadSpec) string {
	parts := []string{workload.Name, workload.Image, workload.Binary}
	parts = append(parts, workload.Command...)
	parts = append(parts, workload.Args...)
	for _, part := range parts {
		value := strings.ToLower(part)
		switch {
		case strings.Contains(value, "btcd"):
			return "btcd"
		case strings.Contains(value, "bitcoind"), strings.Contains(value, "bitcoin-core"), strings.Contains(value, "bitcoin"):
			return "bitcoin-core"
		}
	}
	return ""
}

func findBitcoinWorkload(workloads []v1alpha1.WorkloadSpec) (int, *v1alpha1.WorkloadSpec) {
	for i := range workloads {
		if isBitcoinWorkload(workloads[i]) {
			return i, &workloads[i]
		}
	}
	return -1, nil
}

func isBitcoinWorkload(w v1alpha1.WorkloadSpec) bool {
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

func defaultPortsForNetwork(network string) (int, int) {
	switch normalizeNetwork(network) {
	case "testnet":
		return 18332, 18333
	case "signet":
		return 38332, 38333
	case "regtest":
		return 18443, 18444
	default:
		return 8332, 8333
	}
}

func boolAsInt(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

func normalizeFamily(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "bitcoin", "btc":
		return FamilyName
	default:
		return ""
	}
}

func normalizeClient(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "bitcoind":
		return "bitcoin-core"
	case "bitcoin-core", "btcd":
		return value
	default:
		return value
	}
}

func normalizeNetwork(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "mainnet", "testnet", "signet", "regtest":
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
