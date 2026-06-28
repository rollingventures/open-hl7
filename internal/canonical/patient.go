// Package canonical defines the EMR-agnostic clinical model exchanged between
// EMR adapters and the hub. Adapters map their native model to/from these
// types; the hub maps these to/from HL7 (and, later, FHIR). No type here may
// reference any specific EMR.
package canonical

// Identifier is a single patient identifier (MRN, SSN, an external system id…).
type Identifier struct {
	System string `json:"system"` // e.g. "MRN", "SSN", or an assigning-authority name
	Value  string `json:"value"`
	Type   string `json:"type,omitempty"` // HL7 identifier-type code, e.g. "MR", "SS"
}

// Address is a postal address.
type Address struct {
	Line1   string `json:"line1,omitempty"`
	Line2   string `json:"line2,omitempty"`
	City    string `json:"city,omitempty"`
	State   string `json:"state,omitempty"`
	Zip     string `json:"zip,omitempty"`
	Country string `json:"country,omitempty"`
}

// Source identifies which EMR/facility produced an event.
type Source struct {
	EMR      string `json:"emr"`      // e.g. "openemr"
	Facility string `json:"facility"` // sending facility label
}

// EventType enumerates the patient lifecycle events the hub understands.
type EventType string

const (
	EventCreate EventType = "create" // -> ADT^A04 (register)
	EventUpdate EventType = "update" // -> ADT^A08 (update)
)

// Patient is the neutral patient model. BirthDate is "YYYYMMDD"; Sex is one of
// M, F, O, U (HL7 administrative sex).
type Patient struct {
	MRN         string       `json:"mrn"`
	Identifiers []Identifier `json:"identifiers,omitempty"`
	FamilyName  string       `json:"familyName"`
	GivenName   string       `json:"givenName"`
	MiddleName  string       `json:"middleName,omitempty"`
	BirthDate   string       `json:"birthDate,omitempty"`
	Sex         string       `json:"sex,omitempty"`
	Address     Address      `json:"address,omitempty"`
	Phone       string       `json:"phone,omitempty"`
	Event       EventType    `json:"event"`
	Source      Source       `json:"source"`
}
