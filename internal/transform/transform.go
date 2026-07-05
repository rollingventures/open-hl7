// Package transform runs connector transforms as sandboxed WebAssembly modules.
//
// The declarative field map in internal/connectorgen covers most connectors.
// When a system needs logic a field map can't express — conditional routing,
// custom code-system lookups, odd formatting — a connector can instead carry a
// WASM transform: guest code (compiled from Go/Rust/TinyGo/AssemblyScript) that
// the hub runs in a sandbox with no filesystem and no network. That makes it
// safe to run transforms synthesized by the agentic connector builder, and lets
// connectors be hot-loaded without rebuilding the hub.
package transform

import "context"

// Transformer turns a canonical patient (plus routing context), encoded as
// JSON, into an HL7 message. Implementations must be safe for concurrent use.
type Transformer interface {
	Transform(ctx context.Context, canonicalJSON []byte) ([]byte, error)
}
