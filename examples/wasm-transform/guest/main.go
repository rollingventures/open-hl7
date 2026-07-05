// A sample WASM connector transform for open-hl7.
//
// It reads a canonical patient (plus routing context) as JSON from a static
// input buffer and writes an HL7 ADT message to a static output buffer. The
// point of the spike: this is arbitrary, sandboxed guest code — it applies a
// rule a declarative field map cannot (here, upper-casing the family name) —
// yet the host grants it no filesystem or network access.
//
// ABI (static buffers keep it safe with Go's GC):
//   input_ptr()  -> pointer to the input buffer
//   output_ptr() -> pointer to the output buffer
//   transform(inLen) -> output length, or -1 on error
//
// Build: GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared (see build_guest.sh).
package main

import (
	"encoding/json"
	"strings"
	"unsafe"
)

const bufSize = 64 << 10

var (
	inBuf  [bufSize]byte
	outBuf [bufSize]byte
)

type patient struct {
	MRN        string `json:"mrn"`
	FamilyName string `json:"familyName"`
	GivenName  string `json:"givenName"`
	MiddleName string `json:"middleName"`
	BirthDate  string `json:"birthDate"`
	Sex        string `json:"sex"`
	Event      string `json:"event"`
	ControlID  string `json:"controlId"`
	Timestamp  string `json:"timestamp"`
	Source     struct {
		Facility string `json:"facility"`
	} `json:"source"`
}

//go:wasmexport input_ptr
func inputPtr() int32 { return int32(uintptr(unsafe.Pointer(&inBuf[0]))) }

//go:wasmexport output_ptr
func outputPtr() int32 { return int32(uintptr(unsafe.Pointer(&outBuf[0]))) }

//go:wasmexport transform
func transform(inLen int32) int32 {
	if inLen < 0 || int(inLen) > len(inBuf) {
		return -1
	}
	var p patient
	if err := json.Unmarshal(inBuf[:inLen], &p); err != nil {
		return -1
	}

	trigger := "A04"
	if p.Event == "update" {
		trigger = "A08"
	}
	facility := p.Source.Facility
	if facility == "" {
		facility = "OPENEMR"
	}
	// Custom logic that a static field map can't express: normalize the family
	// name to upper case. (Stand-in for real per-connector rules.)
	family := strings.ToUpper(p.FamilyName)

	var b strings.Builder
	b.WriteString("MSH|^~\\&|OPENEMR|" + facility + "|HUB|HUB|" + p.Timestamp + "||ADT^" + trigger + "|" + p.ControlID + "|P|2.5.1\r")
	b.WriteString("EVN|" + trigger + "|" + p.Timestamp + "\r")
	b.WriteString("PID|1||" + p.MRN + "^^^" + facility + "^MR||" + family + "^" + p.GivenName + "^" + p.MiddleName + "||" + p.BirthDate + "|" + p.Sex + "\r")

	n := copy(outBuf[:], b.String())
	return int32(n)
}

func main() {}
