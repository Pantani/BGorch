package app

import "fmt"

// RuntimeCapability identifies runtime features that may be unsupported by a backend.
type RuntimeCapability string

const (
	// RuntimeCapabilityExecution represents runtime mutation (apply/execute) support.
	RuntimeCapabilityExecution RuntimeCapability = "execution"
	// RuntimeCapabilityObservation represents runtime inspection (status/doctor) support.
	RuntimeCapabilityObservation RuntimeCapability = "observation"
)

// RuntimeUnsupportedError reports that the selected backend does not implement a runtime capability.
type RuntimeUnsupportedError struct {
	Backend    string
	Capability RuntimeCapability
	Required   bool
}

func (e *RuntimeUnsupportedError) Error() string {
	if e.Required {
		return fmt.Sprintf(
			"backend %q does not support runtime %s required by --require-runtime",
			e.Backend,
			e.Capability,
		)
	}
	if e.Capability == RuntimeCapabilityExecution {
		return fmt.Sprintf(
			"backend %q does not support runtime execution; rerun without runtime execution",
			e.Backend,
		)
	}
	return fmt.Sprintf("backend %q does not support runtime %s", e.Backend, e.Capability)
}
