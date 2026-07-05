package channel

import (
	"context"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rollingventures/open-hl7/internal/canonical"
	"github.com/rollingventures/open-hl7/internal/connectorgen"
	"github.com/rollingventures/open-hl7/internal/hl7"
	"github.com/rollingventures/open-hl7/internal/mllp"
	"github.com/rollingventures/open-hl7/internal/store"
	"github.com/rollingventures/open-hl7/internal/transform"
)

// startACKListener runs an MLLP endpoint that ACKs everything, returning its
// address. It picks a free port by binding, closing, and reusing the address.
func startACKListener(t *testing.T, ctx context.Context) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("pick port: %v", err)
	}
	addr := l.Addr().String()
	_ = l.Close()

	srv := &mllp.Server{
		Addr: addr,
		Handler: func(_ context.Context, payload []byte, _ net.Addr) ([]byte, error) {
			msg, _ := hl7.Parse(string(payload))
			return []byte(hl7.BuildACK(msg.Segment("MSH"), hl7.ACKAccept, "").Encode()), nil
		},
	}
	go func() { _ = srv.ListenAndServe(ctx) }()

	// Wait until it accepts connections.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if c, err := net.Dial("tcp", addr); err == nil {
			_ = c.Close()
			return addr
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("listener did not come up on %s", addr)
	return ""
}

func TestRouter_UsesWasmTransformerWhenSet(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	addr := startACKListener(t, ctx)

	w, err := transform.NewWasmTransformer(ctx, nil)
	if err != nil {
		t.Fatalf("wasm: %v", err)
	}
	t.Cleanup(func() { _ = w.Close(ctx) })

	st, err := store.OpenSQLite(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	r := &Router{
		Spec:        connectorgen.OpenEMRADTSpec("wasm-chan", addr),
		Store:       st,
		Transformer: w,
	}

	_, err = r.SendPatient(ctx, canonical.Patient{
		MRN: "55", FamilyName: "Doe", GivenName: "Jane", Sex: "F",
		BirthDate: "19800115", Event: canonical.EventCreate,
		Source: canonical.Source{Facility: "CLINIC"},
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}

	msgs, err := st.List(ctx, 10)
	if err != nil || len(msgs) == 0 {
		t.Fatalf("list: %v (n=%d)", err, len(msgs))
	}
	m := msgs[0]
	// The WASM guest upper-cases the family name — proof the transform ran.
	if !strings.Contains(m.Raw, "DOE^Jane") {
		t.Errorf("expected WASM-transformed PID (DOE^Jane); got:\n%s", m.Raw)
	}
	if m.AckCode != hl7.ACKAccept {
		t.Errorf("expected AA ack, got %q", m.AckCode)
	}
}

// With no transformer, the declarative field map is used (family not upper-cased).
func TestRouter_DefaultsToFieldMap(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	addr := startACKListener(t, ctx)
	st, err := store.OpenSQLite(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	r := &Router{Spec: connectorgen.OpenEMRADTSpec("fm-chan", addr), Store: st}
	if _, err := r.SendPatient(ctx, canonical.Patient{
		MRN: "55", FamilyName: "Doe", GivenName: "Jane", Sex: "F",
		Event: canonical.EventCreate, Source: canonical.Source{Facility: "CLINIC"},
	}); err != nil {
		t.Fatalf("send: %v", err)
	}
	msgs, _ := st.List(ctx, 10)
	if len(msgs) == 0 || !strings.Contains(msgs[0].Raw, "Doe^Jane") {
		t.Errorf("expected field-map PID (Doe^Jane); got: %v", msgs)
	}
}
