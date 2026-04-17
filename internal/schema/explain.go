package schema

import "strings"

// FieldDoc describes one field in the explain output.
type FieldDoc struct {
	Name        string `json:"name" yaml:"name"`
	Type        string `json:"type" yaml:"type"`
	Required    bool   `json:"required" yaml:"required"`
	Description string `json:"description" yaml:"description"`
}

// Doc is the explain payload for a resource path.
type Doc struct {
	Path        string     `json:"path" yaml:"path"`
	Summary     string     `json:"summary" yaml:"summary"`
	Description string     `json:"description" yaml:"description"`
	Fields      []FieldDoc `json:"fields,omitempty" yaml:"fields,omitempty"`
	Examples    []string   `json:"examples,omitempty" yaml:"examples,omitempty"`
	SeeAlso     []string   `json:"seeAlso,omitempty" yaml:"seeAlso,omitempty"`
}

var docs = map[string]Doc{
	"chaincluster": {
		Path:        "ChainCluster",
		Summary:     "Top-level declarative object for a blockchain deployment.",
		Description: "ChainCluster defines desired state for family, plugin, runtime backend, topology, lifecycle policies, and plugin-specific config.",
		Fields: []FieldDoc{
			{Name: "apiVersion", Type: "string", Required: true, Description: "Schema version (bgorch.io/v1alpha1)."},
			{Name: "kind", Type: "string", Required: true, Description: "Object kind (ChainCluster)."},
			{Name: "metadata", Type: "ObjectMeta", Required: true, Description: "Object identity and labels."},
			{Name: "spec", Type: "ChainClusterSpec", Required: true, Description: "Desired runtime topology and policies."},
		},
		Examples: []string{
			"chainops explain ChainCluster.spec",
			"chainops explain ChainCluster.spec.runtime",
		},
	},
	"chaincluster.spec": {
		Path:        "ChainCluster.spec",
		Summary:     "Desired state for chain family, runtime, and lifecycle policies.",
		Description: "spec selects the chain family/plugin, backend runtime, node pools, and typed extension blocks.",
		Fields: []FieldDoc{
			{Name: "family", Type: "string", Required: true, Description: "Logical blockchain family identifier (generic, cometbft, evm, solana, bitcoin, cosmos, ...)."},
			{Name: "plugin", Type: "string", Required: true, Description: "Plugin implementation name for family behavior."},
			{Name: "profile", Type: "string", Required: false, Description: "Optional profile preset name for onboarding/discovery."},
			{Name: "runtime", Type: "RuntimeSpec", Required: true, Description: "Backend and backend-specific settings."},
			{Name: "nodePools", Type: "[]NodePoolSpec", Required: true, Description: "Logical node groups and replicated templates."},
			{Name: "pluginConfig", Type: "PluginConfig", Required: false, Description: "Typed plugin extension block."},
			{Name: "backup", Type: "BackupPolicy", Required: false, Description: "Backup policy declaration."},
			{Name: "upgrade", Type: "UpgradePolicy", Required: false, Description: "Upgrade strategy declaration."},
			{Name: "observe", Type: "ObservePolicy", Required: false, Description: "Observability intent declaration."},
		},
		SeeAlso: []string{"ChainCluster.spec.runtime", "ChainCluster.spec.nodePools"},
	},
	"chaincluster.spec.runtime": {
		Path:        "ChainCluster.spec.runtime",
		Summary:     "Runtime backend selection and backend-specific parameters.",
		Description: "runtime.backend chooses execution backend (compose, ssh-systemd, kubernetes, terraform, ansible). backendConfig contains typed extension blocks.",
		Fields: []FieldDoc{
			{Name: "backend", Type: "string", Required: true, Description: "Execution backend implementation."},
			{Name: "target", Type: "string", Required: false, Description: "Optional target selector for remote backends."},
			{Name: "backendConfig", Type: "BackendConfig", Required: false, Description: "Typed backend extension block."},
		},
		SeeAlso: []string{"plugin generic-process", "plugin cometbft-family", "plugin evm-family", "plugin solana-family", "plugin bitcoin-family", "plugin cosmos-family"},
	},
	"chaincluster.spec.nodepools": {
		Path:        "ChainCluster.spec.nodePools",
		Summary:     "Node topology definition expanded into concrete nodes.",
		Description: "Each nodePool declares replicas and a node template with workloads, volumes, files, and plugin overrides.",
		Fields: []FieldDoc{
			{Name: "name", Type: "string", Required: true, Description: "Unique pool identifier."},
			{Name: "replicas", Type: "int", Required: false, Description: "Replica count. Default: 1."},
			{Name: "roles", Type: "[]string", Required: false, Description: "Optional role labels used by plugins."},
			{Name: "template", Type: "NodeSpec", Required: true, Description: "Node template with workloads and files."},
		},
	},
	"chaincluster.spec.storage": {
		Path:        "ChainCluster.spec.storage",
		Summary:     "Storage intent is modeled at node/workload level.",
		Description: "There is no top-level spec.storage field. Use template.volumes + workload.volumeMounts to model stateful storage and sync policies.",
		Fields: []FieldDoc{
			{Name: "spec.nodePools[].template.volumes", Type: "[]VolumeSpec", Required: false, Description: "Named/bind volume declaration."},
			{Name: "spec.nodePools[].template.workloads[].volumeMounts", Type: "[]VolumeMountSpec", Required: false, Description: "Attach declared volumes to workloads."},
			{Name: "spec.nodePools[].template.sync", Type: "SyncPolicy", Required: false, Description: "Bootstrap/snapshot sync policy."},
		},
		SeeAlso: []string{"ChainCluster.spec.nodePools"},
	},
}

// Lookup finds documentation by explain path, case-insensitive.
func Lookup(path string) (Doc, bool) {
	key := normalize(path)
	doc, ok := docs[key]
	return doc, ok
}

func normalize(path string) string {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "\"")
	path = strings.TrimSuffix(path, "\"")
	return strings.ToLower(path)
}
