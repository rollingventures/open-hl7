// Package connectorgen defines the declarative connector model that the agentic
// connector builder emits and the channel engine consumes. A ConnectorSpec is
// the entire definition of "how to talk to system X" — produced from
// screenshots/samples by a Claude agent (see docs/agentic-connector-builder.md),
// reviewed by a human, then run by the hub. No per-integration code.
package connectorgen

// TransportKind enumerates the supported wire transports.
type TransportKind string

const (
	TransportMLLP TransportKind = "mllp" // TCP + MLLP framing
	TransportSFTP TransportKind = "sftp" // file drop/pickup over SFTP
	TransportFS   TransportKind = "fs"   // local filesystem directory
)

// Transport is how messages move to/from the target system.
type Transport struct {
	Kind    TransportKind `json:"kind"`
	Address string        `json:"address,omitempty"` // host:port (mllp) or host (sftp)
	// Path settings for file transports.
	OutboundPath string `json:"outboundPath,omitempty"`
	InboundPath  string `json:"inboundPath,omitempty"`
	// Credential references resolved at deploy time (never the secrets themselves).
	CredentialRef string `json:"credentialRef,omitempty"`
}

// Direction of the connector relative to the hub.
type Direction string

const (
	DirOutbound      Direction = "outbound"
	DirInbound       Direction = "inbound"
	DirBidirectional Direction = "bidirectional"
)

// FieldMap maps one canonical field to an HL7 location (and back). HL7Path uses
// a simple "SEG-field[.component]" notation, e.g. "PID-5.1" = patient family
// name. Canonical uses dotted canonical paths, e.g. "familyName", "address.city".
type FieldMap struct {
	Canonical  string  `json:"canonical"`
	HL7Path    string  `json:"hl7Path"`
	Transform  string  `json:"transform,omitempty"` // optional named transform (e.g. "date:YYYYMMDD")
	Confidence float64 `json:"confidence"`          // 0..1; low values flagged for human review
	Note       string  `json:"note,omitempty"`
}

// MSHRouting carries the HL7 addressing fields (MSH-3..6, MSH-11).
type MSHRouting struct {
	SendingApp   string `json:"sendingApp"`
	SendingFac   string `json:"sendingFac"`
	ReceivingApp string `json:"receivingApp"`
	ReceivingFac string `json:"receivingFac"`
	Processing   string `json:"processing,omitempty"` // P/T/D
	Version      string `json:"version,omitempty"`    // e.g. 2.5.1
}

// ConnectorSpec is a complete, declarative connector for one target system.
type ConnectorSpec struct {
	Name         string     `json:"name"`
	System       string     `json:"system"`            // e.g. "Epic", "Cerner", "OpenEMR"
	SystemVersion string    `json:"systemVersion,omitempty"`
	Direction    Direction  `json:"direction"`
	MessageTypes []string   `json:"messageTypes"` // e.g. ["ADT^A04","ADT^A08"]
	MSH          MSHRouting `json:"msh"`
	Transport    Transport  `json:"transport"`
	FieldMaps    []FieldMap `json:"fieldMaps"`

	// Provenance from the agent build.
	Confidence float64  `json:"confidence"` // overall 0..1
	Gaps       []string `json:"gaps,omitempty"`
	Notes      string   `json:"notes,omitempty"`
}

// Artifact is one piece of evidence the builder reasons over (a screenshot,
// a sample message, a doc). Data is the raw bytes (e.g. PNG); Kind tells the
// agent how to treat it.
type Artifact struct {
	Kind     string `json:"kind"` // "screenshot" | "sample_message" | "doc"
	Filename string `json:"filename"`
	MediaType string `json:"mediaType"` // e.g. "image/png", "text/plain"
	Data     []byte `json:"-"`
}

// BuildResult is what the agentic builder returns for human review.
type BuildResult struct {
	Spec          ConnectorSpec `json:"spec"`
	SampleMessage string        `json:"sampleMessage"` // a message rendered with the proposed map
	NeedsReview   []string      `json:"needsReview"`   // low-confidence field paths
}

// Builder turns evidence about a target system into a reviewable connector.
// The Claude-backed implementation (vision + tool loop) lands in CB-2/CB-3;
// this interface lets the channel engine and tests depend on the contract now.
type Builder interface {
	Build(artifacts []Artifact, hint Hint) (BuildResult, error)
}

// Hint carries optional operator-supplied context to anchor the agent.
type Hint struct {
	System       string   `json:"system,omitempty"`
	Direction    Direction `json:"direction,omitempty"`
	MessageTypes []string `json:"messageTypes,omitempty"`
}
