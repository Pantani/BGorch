package domain

import "time"

// Severity classifies diagnostics by impact.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Diagnostic represents a user-facing validation or resolution message.
type Diagnostic struct {
	Severity Severity `json:"severity"`
	Path     string   `json:"path,omitempty"`
	Message  string   `json:"message"`
	Hint     string   `json:"hint,omitempty"`
}

// Error builds an error-level diagnostic.
func Error(path, message, hint string) Diagnostic {
	return Diagnostic{Severity: SeverityError, Path: path, Message: message, Hint: hint}
}

// Warning builds a warning-level diagnostic.
func Warning(path, message, hint string) Diagnostic {
	return Diagnostic{Severity: SeverityWarning, Path: path, Message: message, Hint: hint}
}

// Artifact is a rendered file payload relative to an output directory.
type Artifact struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// Service is a backend-agnostic runtime unit derived from workload expansion.
type Service struct {
	Name          string            `json:"name"`
	Node          string            `json:"node"`
	Workload      string            `json:"workload"`
	Image         string            `json:"image,omitempty"`
	Command       []string          `json:"command,omitempty"`
	Args          []string          `json:"args,omitempty"`
	Env           map[string]string `json:"env,omitempty"`
	Ports         []PortBinding     `json:"ports,omitempty"`
	Volumes       []VolumeMount     `json:"volumes,omitempty"`
	RestartPolicy string            `json:"restartPolicy,omitempty"`
	DependsOn     []string          `json:"dependsOn,omitempty"`
	HealthCheck   *HealthCheck      `json:"healthCheck,omitempty"`
}

// PortBinding models one runtime port exposure for a service.
type PortBinding struct {
	ContainerPort int    `json:"containerPort"`
	HostPort      int    `json:"hostPort,omitempty"`
	Protocol      string `json:"protocol,omitempty"`
}

// VolumeMount binds a volume source to a service path.
type VolumeMount struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	Type     string `json:"type,omitempty"`
	ReadOnly bool   `json:"readOnly,omitempty"`
}

// Volume describes a desired runtime volume declaration.
type Volume struct {
	Name     string `json:"name"`
	Driver   string `json:"driver,omitempty"`
	External bool   `json:"external,omitempty"`
}

// Network represents a named runtime network attachment domain.
type Network struct {
	Name string `json:"name"`
}

// HealthCheck captures backend-agnostic probe settings for a service.
type HealthCheck struct {
	Test           []string `json:"test,omitempty"`
	IntervalSec    int      `json:"intervalSec,omitempty"`
	TimeoutSec     int      `json:"timeoutSec,omitempty"`
	Retries        int      `json:"retries,omitempty"`
	StartPeriodSec int      `json:"startPeriodSec,omitempty"`
}

// DesiredState is the normalized output produced by plugin+backend resolution.
type DesiredState struct {
	ClusterName string            `json:"clusterName"`
	Backend     string            `json:"backend"`
	Services    []Service         `json:"services"`
	Volumes     []Volume          `json:"volumes,omitempty"`
	Networks    []Network         `json:"networks,omitempty"`
	Artifacts   []Artifact        `json:"artifacts,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// ChangeType describes desired-vs-current reconciliation intent.
type ChangeType string

const (
	ChangeCreate ChangeType = "create"
	ChangeUpdate ChangeType = "update"
	ChangeDelete ChangeType = "delete"
	ChangeNoop   ChangeType = "noop"
)

// PlanChange is a single diff entry between desired state and snapshot.
type PlanChange struct {
	Type         ChangeType `json:"type"`
	ResourceType string     `json:"resourceType"`
	Name         string     `json:"name"`
	Reason       string     `json:"reason,omitempty"`
}

// Plan is an ordered set of changes generated at a specific UTC timestamp.
type Plan struct {
	GeneratedAt time.Time    `json:"generatedAt"`
	Changes     []PlanChange `json:"changes"`
}

// HasChanges reports whether plan contains non-noop changes.
func (p Plan) HasChanges() bool {
	for _, c := range p.Changes {
		if c.Type != ChangeNoop {
			return true
		}
	}
	return false
}
