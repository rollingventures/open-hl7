package transform

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func sampleInput(event string) []byte {
	in := map[string]any{
		"mrn":        "12345",
		"familyName": "Doe",
		"givenName":  "Jane",
		"middleName": "Q",
		"birthDate":  "19800115",
		"sex":        "F",
		"event":      event,
		"controlId":  "CTRL1",
		"timestamp":  "20260101000000",
		"source":     map[string]string{"facility": "CLINIC"},
	}
	b, _ := json.Marshal(in)
	return b
}

func TestWasmTransform_EncodesADTWithCustomLogic(t *testing.T) {
	ctx := context.Background()
	w, err := NewWasmTransformer(ctx, nil) // embedded sample module
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	t.Cleanup(func() { _ = w.Close(ctx) })

	out, err := w.Transform(ctx, sampleInput("create"))
	if err != nil {
		t.Fatalf("transform: %v", err)
	}
	s := string(out)

	if !strings.Contains(s, "|ADT^A04|CTRL1|") {
		t.Errorf("expected MSH ADT^A04 with control id; got:\n%s", s)
	}
	// The guest applies custom logic a field map can't: upper-case the family name.
	if !strings.Contains(s, "DOE^Jane^Q") {
		t.Errorf("expected upper-cased family name DOE^Jane^Q; got:\n%s", s)
	}
	if !strings.Contains(s, "12345^^^CLINIC^MR") {
		t.Errorf("expected PID-3 with MRN; got:\n%s", s)
	}
}

func TestWasmTransform_UpdateMapsToA08(t *testing.T) {
	ctx := context.Background()
	w, err := NewWasmTransformer(ctx, nil)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	t.Cleanup(func() { _ = w.Close(ctx) })

	out, err := w.Transform(ctx, sampleInput("update"))
	if err != nil {
		t.Fatalf("transform: %v", err)
	}
	if !strings.Contains(string(out), "|ADT^A08|") {
		t.Errorf("expected ADT^A08; got:\n%s", out)
	}
}

// Two calls must succeed against the same compiled module (fresh instance each).
func TestWasmTransform_ReusableAcrossCalls(t *testing.T) {
	ctx := context.Background()
	w, err := NewWasmTransformer(ctx, nil)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	t.Cleanup(func() { _ = w.Close(ctx) })

	for i := 0; i < 3; i++ {
		if _, err := w.Transform(ctx, sampleInput("create")); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
}

func TestWasmTransform_RejectsGarbageInput(t *testing.T) {
	ctx := context.Background()
	w, err := NewWasmTransformer(ctx, nil)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	t.Cleanup(func() { _ = w.Close(ctx) })

	if _, err := w.Transform(ctx, []byte("not json")); err == nil {
		t.Error("expected error on invalid JSON input")
	}
}
