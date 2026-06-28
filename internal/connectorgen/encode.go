package connectorgen

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rollingventures/open-hl7/internal/canonical"
	"github.com/rollingventures/open-hl7/internal/hl7"
)

// CanonicalValues flattens a canonical patient into the dotted-path map that
// FieldMap.Canonical references (e.g. "familyName", "address.city").
func CanonicalValues(p canonical.Patient) map[string]string {
	return map[string]string{
		"mrn":             p.MRN,
		"familyName":      p.FamilyName,
		"givenName":       p.GivenName,
		"middleName":      p.MiddleName,
		"birthDate":       p.BirthDate,
		"sex":             p.Sex,
		"phone":           p.Phone,
		"address.line1":   p.Address.Line1,
		"address.line2":   p.Address.Line2,
		"address.city":    p.Address.City,
		"address.state":   p.Address.State,
		"address.zip":     p.Address.Zip,
		"address.country": p.Address.Country,
		"source.emr":      p.Source.EMR,
		"source.facility": p.Source.Facility,
	}
}

// MessageTypeFor resolves the HL7 message type + trigger for a canonical event,
// and verifies the spec declares it.
func (s ConnectorSpec) MessageTypeFor(e canonical.EventType) (msgType, trigger string, err error) {
	switch e {
	case canonical.EventCreate:
		trigger = "A04"
	case canonical.EventUpdate:
		trigger = "A08"
	default:
		return "", "", fmt.Errorf("connectorgen: unsupported event %q", e)
	}
	msgType = "ADT^" + trigger
	for _, t := range s.MessageTypes {
		if t == msgType {
			return msgType, trigger, nil
		}
	}
	return "", "", fmt.Errorf("connectorgen: spec %q does not declare message type %s", s.Name, msgType)
}

// EncodeMessage renders an HL7 message for a canonical patient by applying the
// spec's field map. MSH and (for ADT) EVN are built structurally from the spec
// routing + runtime clock; all other segments are assembled from FieldMaps,
// which is what lets one engine serve any system without per-system code.
func (s ConnectorSpec) EncodeMessage(p canonical.Patient, e canonical.EventType, controlID string) (string, error) {
	msgType, trigger, err := s.MessageTypeFor(e)
	if err != nil {
		return "", err
	}

	ts := time.Now().UTC().Format("20060102150405")
	vals := CanonicalValues(p)

	segments := [][]string{
		{
			"MSH", `^~\&`,
			s.MSH.SendingApp, s.MSH.SendingFac,
			s.MSH.ReceivingApp, s.MSH.ReceivingFac,
			ts, "", msgType, controlID,
			orDefault(s.MSH.Processing, "P"),
			orDefault(s.MSH.Version, "2.5.1"),
		},
		{"EVN", trigger, ts},
	}

	dataSegs, err := buildDataSegments(s.FieldMaps, vals)
	if err != nil {
		return "", err
	}
	for _, name := range orderedSegmentNames(dataSegs) {
		segments = append(segments, dataSegs[name])
	}

	return hl7.Message{Segments: segments}.Encode(), nil
}

// buildDataSegments materializes non-structural segments from the field map.
func buildDataSegments(maps []FieldMap, vals map[string]string) (map[string][]string, error) {
	// seg -> field -> component -> value
	grid := map[string]map[int]map[int]string{}

	for _, fm := range maps {
		seg, field, comp, err := parseHL7Path(fm.HL7Path)
		if err != nil {
			return nil, err
		}
		if seg == "MSH" || seg == "EVN" {
			continue // structural segments are owned by EncodeMessage
		}
		val := resolveValue(fm.Canonical, vals)
		val = applyTransform(fm.Transform, val)

		if grid[seg] == nil {
			grid[seg] = map[int]map[int]string{}
		}
		if grid[seg][field] == nil {
			grid[seg][field] = map[int]string{}
		}
		grid[seg][field][comp] = val
	}

	out := map[string][]string{}
	for seg, fields := range grid {
		maxField := 0
		for f := range fields {
			if f > maxField {
				maxField = f
			}
		}
		arr := make([]string, maxField+1)
		arr[0] = seg
		for f, comps := range fields {
			maxComp := 0
			for c := range comps {
				if c > maxComp {
					maxComp = c
				}
			}
			parts := make([]string, maxComp)
			for c := 1; c <= maxComp; c++ {
				parts[c-1] = comps[c]
			}
			arr[f] = strings.Join(parts, "^")
		}
		out[seg] = arr
	}
	return out, nil
}

// orderedSegmentNames returns data-segment names in canonical HL7 order, with
// any unknown segments appended alphabetically (deterministic output).
func orderedSegmentNames(segs map[string][]string) []string {
	order := map[string]int{"PID": 1, "PD1": 2, "NK1": 3, "PV1": 4, "PV2": 5, "OBR": 6, "OBX": 7}
	names := make([]string, 0, len(segs))
	for n := range segs {
		names = append(names, n)
	}
	sort.SliceStable(names, func(i, j int) bool {
		oi, iok := order[names[i]]
		oj, jok := order[names[j]]
		if iok && jok {
			return oi < oj
		}
		if iok != jok {
			return iok // known segments first
		}
		return names[i] < names[j]
	})
	return names
}

// parseHL7Path parses "SEG-field[.component]" (1-based; component defaults to 1).
func parseHL7Path(path string) (seg string, field, comp int, err error) {
	dash := strings.IndexByte(path, '-')
	if dash < 1 {
		return "", 0, 0, fmt.Errorf("connectorgen: invalid HL7 path %q", path)
	}
	seg = path[:dash]
	rest := path[dash+1:]

	fieldStr, compStr := rest, "1"
	if dot := strings.IndexByte(rest, '.'); dot >= 0 {
		fieldStr, compStr = rest[:dot], rest[dot+1:]
	}
	if field, err = strconv.Atoi(fieldStr); err != nil || field < 1 {
		return "", 0, 0, fmt.Errorf("connectorgen: invalid field in path %q", path)
	}
	if comp, err = strconv.Atoi(compStr); err != nil || comp < 1 {
		return "", 0, 0, fmt.Errorf("connectorgen: invalid component in path %q", path)
	}
	return seg, field, comp, nil
}

// resolveValue returns a literal (Canonical starting with "=") or a looked-up
// canonical value.
func resolveValue(canonicalPath string, vals map[string]string) string {
	if strings.HasPrefix(canonicalPath, "=") {
		return canonicalPath[1:]
	}
	return vals[canonicalPath]
}

func applyTransform(name, v string) string {
	switch name {
	case "", "identity":
		return v
	case "upper":
		return strings.ToUpper(v)
	case "lower":
		return strings.ToLower(v)
	default:
		return v
	}
}

func orDefault(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

// OpenEMRADTSpec is the hand-authored reference connector for OpenEMR ADT — the
// high-confidence baseline the agentic builder is evaluated against, and the
// default the hub uses when no spec file is supplied.
func OpenEMRADTSpec(name, destination string) ConnectorSpec {
	return ConnectorSpec{
		Name:         name,
		System:       "OpenEMR",
		Direction:    DirOutbound,
		MessageTypes: []string{"ADT^A04", "ADT^A08"},
		MSH: MSHRouting{
			SendingApp: "OPENEMR", SendingFac: "OPENEMR",
			ReceivingApp: "HUB", ReceivingFac: "HUB",
			Processing: "P", Version: "2.5.1",
		},
		Transport: Transport{Kind: TransportMLLP, Address: destination},
		FieldMaps: []FieldMap{
			{Canonical: "=1", HL7Path: "PID-1.1", Confidence: 1},
			{Canonical: "mrn", HL7Path: "PID-3.1", Confidence: 1},
			{Canonical: "source.facility", HL7Path: "PID-3.4", Confidence: 0.9},
			{Canonical: "=MR", HL7Path: "PID-3.5", Confidence: 1},
			{Canonical: "familyName", HL7Path: "PID-5.1", Confidence: 1},
			{Canonical: "givenName", HL7Path: "PID-5.2", Confidence: 1},
			{Canonical: "middleName", HL7Path: "PID-5.3", Confidence: 0.9},
			{Canonical: "birthDate", HL7Path: "PID-7.1", Confidence: 1},
			{Canonical: "sex", HL7Path: "PID-8.1", Confidence: 1},
			{Canonical: "address.line1", HL7Path: "PID-11.1", Confidence: 0.9},
			{Canonical: "address.city", HL7Path: "PID-11.3", Confidence: 0.9},
			{Canonical: "address.state", HL7Path: "PID-11.4", Confidence: 0.9},
			{Canonical: "address.zip", HL7Path: "PID-11.5", Confidence: 0.9},
			{Canonical: "address.country", HL7Path: "PID-11.6", Confidence: 0.8},
			{Canonical: "phone", HL7Path: "PID-13.1", Confidence: 0.8},
		},
		Confidence: 0.95,
	}
}
