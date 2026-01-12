package runtime

import (
	"fmt"
	"strings"
)

// RuntimeType represents the type of runtime (Python, Node, Bun, etc.)
type RuntimeType string

const (
	// RuntimePython represents Python interpreter
	RuntimePython RuntimeType = "python"

	// RuntimeNode represents Node.js runtime (future)
	RuntimeNode RuntimeType = "node"

	// RuntimeBun represents Bun runtime (future)
	RuntimeBun RuntimeType = "bun"

	// RuntimeDeno represents Deno runtime (future)
	RuntimeDeno RuntimeType = "deno"
)

// String returns the string representation of the runtime type
func (rt RuntimeType) String() string {
	return string(rt)
}

// IsValid checks if the runtime type is supported
func (rt RuntimeType) IsValid() bool {
	switch rt {
	case RuntimePython, RuntimeNode, RuntimeBun, RuntimeDeno:
		return true
	default:
		return false
	}
}

// IsImplemented checks if the runtime type is currently implemented
func (rt RuntimeType) IsImplemented() bool {
	switch rt {
	case RuntimePython, RuntimeNode:
		return true
	case RuntimeBun, RuntimeDeno:
		return false
	default:
		return false
	}
}

// RuntimeSpec represents a runtime specification with type and version
type RuntimeSpec struct {
	Type    RuntimeType
	Version string
}

// ParseRuntimeSpec parses a runtime specification string
// Formats supported:
// - "python@3.12.1" - explicit runtime type
// - "node@20.0.0" - explicit runtime type
// - "3.12.1" - defaults to Python for backward compatibility
// - "20.0.0" - defaults to Python (but this could be ambiguous!)
func ParseRuntimeSpec(spec string) (*RuntimeSpec, error) {
	// Check for runtime@version format
	if strings.Contains(spec, "@") {
		parts := strings.SplitN(spec, "@", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid runtime specification: %s (expected format: runtime@version)", spec)
		}

		runtimeType := RuntimeType(strings.ToLower(strings.TrimSpace(parts[0])))
		version := strings.TrimSpace(parts[1])

		if version == "" {
			return nil, fmt.Errorf("version cannot be empty in specification: %s", spec)
		}

		if !runtimeType.IsValid() {
			return nil, fmt.Errorf("unsupported runtime type: %s (supported: python, node, bun, deno)", runtimeType)
		}

		if !runtimeType.IsImplemented() {
			return nil, fmt.Errorf("runtime type not yet implemented: %s (currently only Python is supported)", runtimeType)
		}

		return &RuntimeSpec{
			Type:    runtimeType,
			Version: version,
		}, nil
	}

	// No @ symbol - assume Python for backward compatibility
	version := strings.TrimSpace(spec)
	if version == "" {
		return nil, fmt.Errorf("version cannot be empty")
	}

	return &RuntimeSpec{
		Type:    RuntimePython,
		Version: version,
	}, nil
}

// String returns the string representation of the runtime spec
func (rs *RuntimeSpec) String() string {
	return fmt.Sprintf("%s@%s", rs.Type, rs.Version)
}

// DisplayName returns a human-readable name for the runtime
func (rt RuntimeType) DisplayName() string {
	switch rt {
	case RuntimePython:
		return "Python"
	case RuntimeNode:
		return "Node.js"
	case RuntimeBun:
		return "Bun"
	case RuntimeDeno:
		return "Deno"
	default:
		return string(rt)
	}
}
