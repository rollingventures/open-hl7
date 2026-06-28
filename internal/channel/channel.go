// Package channel is the routing core: it turns a canonical event into an HL7
// message, sends it to a destination over MLLP, records both the message and
// the ACK, and conversely decodes inbound MLLP messages back to canonical and
// records them. This is the generic, message-type-agnostic dispatch that
// OpenEMR's per-vendor (quest_ln/labcorp_ln) code never had.
package channel

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/rollingventures/hl7-hub/internal/canonical"
	"github.com/rollingventures/hl7-hub/internal/hl7"
	"github.com/rollingventures/hl7-hub/internal/mllp"
	"github.com/rollingventures/hl7-hub/internal/store"
)

// Config describes one channel (M1: a single ADT channel).
type Config struct {
	Name         string `yaml:"name"`
	DestinationAddr string `yaml:"destinationAddr"` // host:port of the downstream MLLP listener
	SendingApp   string `yaml:"sendingApp"`
	SendingFac   string `yaml:"sendingFac"`
	ReceivingApp string `yaml:"receivingApp"`
	ReceivingFac string `yaml:"receivingFac"`
	SendTimeout  time.Duration `yaml:"sendTimeout"`
}

// Router wires a channel config to the store.
type Router struct {
	Cfg    Config
	Store  store.Store
	Logger *slog.Logger
	seq    int64
}

func (r *Router) log() *slog.Logger {
	if r.Logger != nil {
		return r.Logger
	}
	return slog.Default()
}

func (r *Router) nextCtrl() string {
	r.seq++
	return hl7.NewControlID(fmt.Sprintf("%04d", r.seq%10000))
}

// SendPatient encodes a canonical patient as ADT, sends it to the destination
// over MLLP, persists it, and records the ACK result. Returns the stored id.
func (r *Router) SendPatient(ctx context.Context, p canonical.Patient) (int64, error) {
	ctrl := r.nextCtrl()
	route := hl7.MSHHeader{
		SendingApp:   r.Cfg.SendingApp,
		SendingFac:   r.Cfg.SendingFac,
		ReceivingApp: r.Cfg.ReceivingApp,
		ReceivingFac: r.Cfg.ReceivingFac,
	}
	raw, err := hl7.EncodeADT(p, route, ctrl)
	if err != nil {
		return 0, fmt.Errorf("encode adt: %w", err)
	}

	msgType := "ADT^A04"
	if p.Event == canonical.EventUpdate {
		msgType = "ADT^A08"
	}
	id, err := r.Store.Save(ctx, store.Message{
		Channel: r.Cfg.Name, Direction: store.Outbound, Type: msgType,
		ControlID: ctrl, Raw: raw, Status: "sent",
	})
	if err != nil {
		return 0, err
	}

	timeout := r.Cfg.SendTimeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	ackRaw, err := mllp.Send(r.Cfg.DestinationAddr, []byte(raw), timeout)
	if err != nil {
		_ = r.Store.SetAck(ctx, id, "", err.Error(), "error")
		return id, fmt.Errorf("mllp send: %w", err)
	}

	code, text := parseACK(string(ackRaw))
	status := "acked"
	if code != hl7.ACKAccept {
		status = "nacked"
	}
	if err := r.Store.SetAck(ctx, id, code, text, status); err != nil {
		r.log().Error("failed to record ack", "id", id, "err", err)
	}
	r.log().Info("sent ADT", "id", id, "type", msgType, "ack", code)
	return id, nil
}

// InboundHandler is the mllp.Handler: it decodes an inbound ADT, persists it,
// and returns an ACK. Writing the patient into a target EMR is delegated to
// the (optional) Sink — for OpenEMR that is an HTTP POST to the module webhook.
func (r *Router) InboundHandler(sink Sink) mllp.Handler {
	return func(ctx context.Context, payload []byte, remote net.Addr) ([]byte, error) {
		raw := string(payload)
		dec, err := hl7.DecodeADT(raw)
		if err != nil {
			r.log().Error("decode inbound adt", "err", err, "remote", remote.String())
			msg, _ := hl7.Parse(raw)
			return []byte(hl7.BuildACK(msg.Segment("MSH"), hl7.ACKError, "cannot parse ADT").Encode()), err
		}

		id, serr := r.Store.Save(ctx, store.Message{
			Channel: r.Cfg.Name, Direction: store.Inbound,
			Type: "ADT^" + dec.Trigger, ControlID: dec.ControlID, Raw: raw, Status: "received",
		})
		if serr != nil {
			r.log().Error("persist inbound", "err", serr)
		}

		ackCode := hl7.ACKAccept
		ackText := ""
		if sink != nil {
			if err := sink.WritePatient(ctx, dec.Patient); err != nil {
				ackCode, ackText = hl7.ACKError, "downstream write failed"
				r.log().Error("sink write failed", "err", err)
				_ = r.Store.SetAck(ctx, id, ackCode, ackText, "error")
			}
		}
		if ackCode == hl7.ACKAccept {
			_ = r.Store.SetAck(ctx, id, ackCode, "", "acked")
		}
		return []byte(hl7.BuildACK(dec.MSH, ackCode, ackText).Encode()), nil
	}
}

// Sink receives patients decoded from inbound messages (e.g. the OpenEMR
// adapter webhook). Nil sink = store-and-ack only.
type Sink interface {
	WritePatient(ctx context.Context, p canonical.Patient) error
}

func parseACK(raw string) (code, text string) {
	msg, err := hl7.Parse(raw)
	if err != nil {
		return hl7.ACKError, "unparseable ack"
	}
	msa := msg.Segment("MSA")
	if msa == nil {
		return hl7.ACKError, "no MSA"
	}
	return hl7.Field(msa, 1), hl7.Field(msa, 3)
}
