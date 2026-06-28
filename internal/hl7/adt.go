package hl7

import (
	"fmt"
	"strings"

	"github.com/rollingventures/open-hl7/internal/canonical"
)

// triggerForEvent maps a canonical event to its ADT trigger event.
func triggerForEvent(e canonical.EventType) (string, error) {
	switch e {
	case canonical.EventCreate:
		return "A04", nil // register a patient
	case canonical.EventUpdate:
		return "A08", nil // update patient information
	default:
		return "", fmt.Errorf("hl7: unsupported event type %q", e)
	}
}

// EncodeADT builds an ADT^Axx message (MSH/EVN/PID) from a canonical patient.
// ctrlID is the message control id; route carries the MSH addressing.
func EncodeADT(p canonical.Patient, route MSHHeader, ctrlID string) (string, error) {
	trigger, err := triggerForEvent(p.Event)
	if err != nil {
		return "", err
	}
	route.MessageType = "ADT^" + trigger
	route.ControlID = ctrlID
	if route.SendingApp == "" {
		route.SendingApp = "OPENEMR"
	}
	if route.SendingFac == "" {
		route.SendingFac = p.Source.Facility
	}

	msh := buildMSH(route)
	evn := []string{"EVN", trigger, evnNow()}
	pid := buildPID(p)

	return Message{Segments: [][]string{msh, evn, pid}}.Encode(), nil
}

func evnNow() string {
	// EVN-2 recorded date/time; reuse the MSH timestamp format.
	return NewControlID("")[:14]
}

// buildPID assembles a PID segment from canonical fields.
func buildPID(p canonical.Patient) []string {
	// PID-3: patient identifier list. Primary = MRN with type MR, plus extras.
	ids := []string{}
	if p.MRN != "" {
		ids = append(ids, p.MRN+"^^^"+def(p.Source.Facility, "OPENEMR")+"^MR")
	}
	for _, id := range p.Identifiers {
		ids = append(ids, strings.Join([]string{id.Value, "", "", id.System, def(id.Type, "")}, compSep))
	}
	pid3 := strings.Join(ids, repSep)

	// PID-5: name = family^given^middle
	name := strings.Join([]string{p.FamilyName, p.GivenName, p.MiddleName}, compSep)

	// PID-11: address = line1^line2^city^state^zip^country
	addr := strings.Join([]string{
		p.Address.Line1, p.Address.Line2, p.Address.City,
		p.Address.State, p.Address.Zip, p.Address.Country,
	}, compSep)

	return []string{
		"PID",
		"1",        // PID-1 set id
		"",         // PID-2 (deprecated)
		pid3,       // PID-3 identifier list
		"",         // PID-4 (deprecated)
		name,       // PID-5 name
		"",         // PID-6 mother's maiden name
		p.BirthDate, // PID-7 DOB
		p.Sex,      // PID-8 administrative sex
		"",         // PID-9
		"",         // PID-10 race
		addr,       // PID-11 address
		"",         // PID-12 county
		p.Phone,    // PID-13 home phone
	}
}

// DecodedADT is the result of parsing an inbound ADT message.
type DecodedADT struct {
	Patient   canonical.Patient
	Trigger   string
	ControlID string
	MSH       []string
}

// DecodeADT parses an inbound ADT message into a canonical patient. It extracts
// the MRN (first PID-3 repetition), name, DOB, sex, address and phone.
func DecodeADT(raw string) (DecodedADT, error) {
	msg, err := Parse(raw)
	if err != nil {
		return DecodedADT{}, err
	}
	msh := msg.Segment("MSH")
	pid := msg.Segment("PID")
	if pid == nil {
		return DecodedADT{}, fmt.Errorf("hl7: ADT has no PID segment")
	}

	trigger := Component(Field(msh, 9), 2) // MSH-9.2 trigger event
	p := canonical.Patient{
		Source: canonical.Source{EMR: "external", Facility: Field(msh, 4)},
	}
	switch trigger {
	case "A08", "A31":
		p.Event = canonical.EventUpdate
	default:
		p.Event = canonical.EventCreate
	}

	pid3 := strings.Split(Field(pid, 3), repSep)
	if len(pid3) > 0 {
		p.MRN = Component(pid3[0], 1)
	}
	name := Field(pid, 5)
	p.FamilyName = Component(name, 1)
	p.GivenName = Component(name, 2)
	p.MiddleName = Component(name, 3)
	p.BirthDate = Field(pid, 7)
	p.Sex = Field(pid, 8)
	addr := Field(pid, 11)
	p.Address = canonical.Address{
		Line1:   Component(addr, 1),
		Line2:   Component(addr, 2),
		City:    Component(addr, 3),
		State:   Component(addr, 4),
		Zip:     Component(addr, 5),
		Country: Component(addr, 6),
	}
	p.Phone = Field(pid, 13)

	return DecodedADT{Patient: p, Trigger: trigger, ControlID: Field(msh, 10), MSH: msh}, nil
}
