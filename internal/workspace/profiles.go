package workspace

import (
	"fmt"
	"strings"
)

// Profile defines an onboarding preset.
type Profile struct {
	Name          string `json:"name" yaml:"name"`
	Summary       string `json:"summary" yaml:"summary"`
	Family        string `json:"family" yaml:"family"`
	Plugin        string `json:"plugin" yaml:"plugin"`
	Backend       string `json:"backend" yaml:"backend"`
	IntendedUsers string `json:"intendedUsers" yaml:"intendedUsers"`
}

var builtinProfiles = []Profile{
	{
		Name:          "local-dev",
		Summary:       "Single-node local stack for rapid development and teardown.",
		Family:        "generic",
		Plugin:        "generic-process",
		Backend:       "docker-compose",
		IntendedUsers: "Local developer",
	},
	{
		Name:          "compose-single",
		Summary:       "Single-node compose deployment with explicit network + volumes.",
		Family:        "generic",
		Plugin:        "generic-process",
		Backend:       "docker-compose",
		IntendedUsers: "Operator / DevOps",
	},
	{
		Name:          "vm-single",
		Summary:       "Single-node host-mode deployment via ssh + systemd.",
		Family:        "generic",
		Plugin:        "generic-process",
		Backend:       "ssh-systemd",
		IntendedUsers: "Operator / SRE",
	},
	{
		Name:          "cometbft-local",
		Summary:       "CometBFT validator starter for local testnets.",
		Family:        "cometbft",
		Plugin:        "cometbft-family",
		Backend:       "docker-compose",
		IntendedUsers: "Blockchain operator",
	},
}

// Profiles returns built-in onboarding profiles.
func Profiles() []Profile {
	out := make([]Profile, 0, len(builtinProfiles))
	out = append(out, builtinProfiles...)
	return out
}

// GetProfile resolves a profile by name.
func GetProfile(name string) (Profile, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, p := range builtinProfiles {
		if p.Name == name {
			return p, true
		}
	}
	return Profile{}, false
}

// InitRequest contains onboarding inputs.
type InitRequest struct {
	ClusterName string
	Profile     string
	Family      string
	Plugin      string
	Backend     string
}

// BuildSpec generates a starter chainops spec with inline guidance comments.
func BuildSpec(req InitRequest) (string, error) {
	profileName := strings.ToLower(strings.TrimSpace(req.Profile))
	if profileName == "" {
		profileName = "local-dev"
	}

	profile, ok := GetProfile(profileName)
	if !ok {
		return "", fmt.Errorf("unknown profile %q", req.Profile)
	}

	clusterName := sanitizeName(req.ClusterName)
	if clusterName == "" {
		clusterName = "chainops-local"
	}
	family := firstNonEmpty(req.Family, profile.Family)
	plugin := firstNonEmpty(req.Plugin, profile.Plugin)
	backend := firstNonEmpty(req.Backend, profile.Backend)

	switch backend {
	case "docker-compose", "compose":
		return renderComposeSpec(clusterName, profile.Name, family, plugin), nil
	case "ssh-systemd", "sshsystemd":
		return renderSSHSpec(clusterName, profile.Name, family, plugin), nil
	default:
		return "", fmt.Errorf("profile backend %q is not supported by init templates yet", backend)
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func sanitizeName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	repl := strings.NewReplacer(" ", "-", "_", "-", "/", "-", ".", "-")
	name = repl.Replace(name)
	name = strings.Trim(name, "-")
	return name
}

func renderComposeSpec(clusterName, profileName, family, plugin string) string {
	return fmt.Sprintf(`apiVersion: bgorch.io/v1alpha1
kind: ChainCluster
metadata:
  name: %s
spec:
  # Family = blockchain family identifier (generic/cometbft/...).
  family: %s
  # Profile documents intended shape, useful for explain/profile commands.
  profile: %s
  plugin: %s
  runtime:
    backend: docker-compose
    backendConfig:
      compose:
        projectName: %s
        outputFile: compose.yaml
  nodePools:
    - name: validator
      replicas: 1
      roles: [validator]
      template:
        workloads:
          - name: node
            mode: container
            image: ghcr.io/example/chaind:v0.1.0
            command: ["chaind"]
            args: ["start", "--home", "/var/lib/chain"]
            ports:
              - name: p2p
                containerPort: 26656
                hostPort: 26656
              - name: rpc
                containerPort: 26657
                hostPort: 26657
            volumeMounts:
              - volume: datadir
                path: /var/lib/chain
            restartPolicy: unless-stopped
        volumes:
          - name: datadir
            type: named
`, clusterName, family, profileName, plugin, clusterName)
}

func renderSSHSpec(clusterName, profileName, family, plugin string) string {
	return fmt.Sprintf(`apiVersion: bgorch.io/v1alpha1
kind: ChainCluster
metadata:
  name: %s
spec:
  family: %s
  profile: %s
  plugin: %s
  runtime:
    backend: ssh-systemd
    # target can reference inventory/group labels for remote ops.
    target: validators
    backendConfig:
      sshSystemd:
        user: chainops
        port: 22
  nodePools:
    - name: validator
      replicas: 1
      roles: [validator]
      template:
        workloads:
          - name: node
            mode: host
            binary: /usr/local/bin/chaind
            args: ["start", "--home", "/var/lib/chain"]
            restartPolicy: always
            volumeMounts:
              - volume: datadir
                path: /var/lib/chain
        volumes:
          - name: datadir
            type: bind
            source: /var/lib/chain
`, clusterName, family, profileName, plugin)
}
