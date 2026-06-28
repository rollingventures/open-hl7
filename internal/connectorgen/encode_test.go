package connectorgen

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rollingventures/open-hl7/internal/canonical"
)

func samplePatient(event canonical.EventType) canonical.Patient {
	return canonical.Patient{
		MRN:        "12345",
		FamilyName: "Doe",
		GivenName:  "Jane",
		BirthDate:  "19800115",
		Sex:        "F",
		Event:      event,
		Source:     canonical.Source{EMR: "openemr", Facility: "CLINIC"},
	}
}

func TestEncodeMessage_ADTCreateBuildsExpectedPID(t *testing.T) {
	spec := OpenEMRADTSpec("openemr-adt", "127.0.0.1:2576")
	raw, err := spec.EncodeMessage(samplePatient(canonical.EventCreate), canonical.EventCreate, "CTRL1")
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	if !strings.Contains(raw, "|ADT^A04|CTRL1|") {
		t.Errorf("expected MSH message type ADT^A04 with control id; got:\n%s", raw)
	}
	const wantPID = "PID|1||12345^^^CLINIC^MR||Doe^Jane^||19800115|F|||^^^^^||"
	if !strings.Contains(raw, wantPID) {
		t.Errorf("PID segment mismatch.\nwant substring: %q\ngot:\n%s", wantPID, raw)
	}
	if !strings.Contains(raw, "EVN|A04|") {
		t.Errorf("expected EVN|A04; got:\n%s", raw)
	}
}

func TestEncodeMessage_UpdateMapsToA08(t *testing.T) {
	spec := OpenEMRADTSpec("openemr-adt", "127.0.0.1:2576")
	raw, err := spec.EncodeMessage(samplePatient(canonical.EventUpdate), canonical.EventUpdate, "CTRL2")
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if !strings.Contains(raw, "|ADT^A08|") || !strings.Contains(raw, "EVN|A08|") {
		t.Errorf("expected ADT^A08 / EVN|A08; got:\n%s", raw)
	}
}

func TestMessageTypeFor_UndeclaredTypeErrors(t *testing.T) {
	spec := OpenEMRADTSpec("only-create", "x:1")
	spec.MessageTypes = []string{"ADT^A04"} // no A08
	if _, _, err := spec.MessageTypeFor(canonical.EventUpdate); err == nil {
		t.Error("expected error for undeclared ADT^A08, got nil")
	}
}

func TestLoadSpec_RoundTripsReference(t *testing.T) {
	ref := OpenEMRADTSpec("openemr-adt", "127.0.0.1:2576")
	data, err := json.MarshalIndent(ref, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	path := filepath.Join(t.TempDir(), "spec.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	loaded, err := LoadSpec(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Name != ref.Name || loaded.System != ref.System {
		t.Errorf("identity mismatch: got %q/%q", loaded.Name, loaded.System)
	}
	if len(loaded.FieldMaps) != len(ref.FieldMaps) {
		t.Errorf("field map count: got %d want %d", len(loaded.FieldMaps), len(ref.FieldMaps))
	}

	// A loaded spec must encode identically to the in-code reference.
	a, _ := ref.EncodeMessage(samplePatient(canonical.EventCreate), canonical.EventCreate, "C")
	b, _ := loaded.EncodeMessage(samplePatient(canonical.EventCreate), canonical.EventCreate, "C")
	if a != b {
		t.Errorf("loaded spec encodes differently:\nref:    %q\nloaded: %q", a, b)
	}
}

func TestValidate_RejectsBadPath(t *testing.T) {
	spec := OpenEMRADTSpec("bad", "x:1")
	spec.FieldMaps = append(spec.FieldMaps, FieldMap{Canonical: "mrn", HL7Path: "PID5"})
	if err := spec.Validate(); err == nil {
		t.Error("expected validation error for malformed HL7 path")
	}
}
