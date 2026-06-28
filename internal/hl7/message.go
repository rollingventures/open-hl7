// Package hl7 is a deliberately small HL7 v2.x encoder/decoder — enough for the
// M1 ADT slice (MSH/EVN/PID) plus ACK (MSH/MSA). It is NOT a full conformance
// engine; it handles the standard encoding characters |^~\& and assumes the
// default field/component/repetition/escape/subcomponent separators.
package hl7

import (
	"fmt"
	"strings"
	"time"
)

const (
	fieldSep = "|"
	compSep  = "^"
	repSep   = "~"
)

// Message is a parsed HL7 message: an ordered list of segments, each a list of
// fields. Field 0 of every segment is the segment name (e.g. "MSH").
type Message struct {
	Segments [][]string
}

// Segment returns the first segment with the given name, or nil.
func (m Message) Segment(name string) []string {
	for _, seg := range m.Segments {
		if len(seg) > 0 && seg[0] == name {
			return seg
		}
	}
	return nil
}

// Field returns segment.field (1-based field index, HL7 style). For MSH, field
// 1 is the field separator and field 2 the encoding chars, matching the wire.
func Field(seg []string, idx int) string {
	if idx < 0 || idx >= len(seg) {
		return ""
	}
	return seg[idx]
}

// Component returns the n-th ^-component (1-based) of a field value.
func Component(field string, n int) string {
	parts := strings.Split(field, compSep)
	if n < 1 || n > len(parts) {
		return ""
	}
	return parts[n-1]
}

// Encode renders a message to its wire string (segments joined by \r). MSH is
// special-cased so MSH-1/MSH-2 (the separators themselves) serialize correctly.
func (m Message) Encode() string {
	var b strings.Builder
	for _, seg := range m.Segments {
		if len(seg) == 0 {
			continue
		}
		if seg[0] == "MSH" {
			// seg = ["MSH", "^~\\&", field3, field4, ...]; the field separator
			// is implied between MSH and the encoding chars.
			b.WriteString("MSH")
			b.WriteString(fieldSep)
			b.WriteString(strings.Join(seg[1:], fieldSep))
		} else {
			b.WriteString(strings.Join(seg, fieldSep))
		}
		b.WriteString("\r")
	}
	return b.String()
}

// Parse parses a raw HL7 message. Segments may be separated by \r, \n, or \r\n.
func Parse(raw string) (Message, error) {
	norm := strings.NewReplacer("\r\n", "\r", "\n", "\r").Replace(raw)
	lines := strings.Split(strings.Trim(norm, "\r"), "\r")
	var msg Message
	for _, line := range lines {
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "MSH") {
			// Re-attach the encoding chars as field 1 after the name so callers
			// can index MSH fields by their HL7 numbers.
			rest := strings.Split(line[4:], fieldSep) // drop "MSH|"
			seg := append([]string{"MSH"}, rest...)
			msg.Segments = append(msg.Segments, seg)
			continue
		}
		msg.Segments = append(msg.Segments, strings.Split(line, fieldSep))
	}
	if len(msg.Segments) == 0 || msg.Segments[0][0] != "MSH" {
		return Message{}, fmt.Errorf("hl7: message does not start with MSH")
	}
	return msg, nil
}

// MSHHeader carries the routing fields needed to build a message or its ACK.
type MSHHeader struct {
	SendingApp  string
	SendingFac  string
	ReceivingApp string
	ReceivingFac string
	MessageType string // e.g. "ADT^A04" or "ADT^A04^ADT_A01"
	ControlID   string
	Processing  string // "P" prod, "T" test, "D" debug
	Version     string // e.g. "2.5.1"
}

// NewControlID returns a message control id derived from the clock plus a
// caller-supplied suffix for uniqueness within the same second.
func NewControlID(suffix string) string {
	return time.Now().UTC().Format("20060102150405") + suffix
}

func buildMSH(h MSHHeader) []string {
	ts := time.Now().UTC().Format("20060102150405")
	return []string{
		"MSH",
		`^~\&`,
		h.SendingApp,
		h.SendingFac,
		h.ReceivingApp,
		h.ReceivingFac,
		ts,
		"", // MSH-8 security
		h.MessageType,
		h.ControlID,
		def(h.Processing, "P"),
		def(h.Version, "2.5.1"),
	}
}

func def(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

// ACK codes.
const (
	ACKAccept = "AA"
	ACKError  = "AE"
	ACKReject = "AR"
)

// BuildACK builds an ACK for an inbound message, echoing its control id in MSA-2
// and swapping sender/receiver. inMSH is the parsed inbound MSH segment.
func BuildACK(inMSH []string, code, text string) Message {
	// Inbound (HL7-numbered): 3=sendApp 4=sendFac 5=recvApp 6=recvFac 10=ctrlID 12=version
	h := MSHHeader{
		SendingApp:   Field(inMSH, 5),
		SendingFac:   Field(inMSH, 6),
		ReceivingApp: Field(inMSH, 3),
		ReceivingFac: Field(inMSH, 4),
		MessageType:  "ACK",
		ControlID:    NewControlID("A"),
		Processing:   Field(inMSH, 11),
		Version:      Field(inMSH, 12),
	}
	msa := []string{"MSA", code, Field(inMSH, 10)}
	if text != "" {
		msa = append(msa, text)
	}
	return Message{Segments: [][]string{buildMSH(h), msa}}
}
