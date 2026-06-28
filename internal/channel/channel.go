// Package channel is the routing core: it turns a canonical event into an HL7
// message *using a declarative ConnectorSpec*, sends it to a destination over
// MLLP, records both the message and the ACK, and conversely decodes inbound
// MLLP messages back to canonical and records them. Because encoding is driven
// by the spec's field map (not hard-coded per vendor), one engine serves any
// system — including specs synthesized by the agentic connector builder.
package channel

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/rollingventures/open-hl7/internal/canonical"
	"github.com/rollingventures/open-hl7/internal/connectorgen"
	"github.com/rollingventures/open-hl7/internal/hl7"
	"github.com/rollingventures/open-hl7/internal/mllp"
	"github.com/rollingventures/open-hl7/internal/store"
)

const defaultSendTimeout = 10 * time.Second

// Router runs one ConnectorSpec against the store.
type Router struct {
	Spec   connectorgen.ConnectorSpec
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

// SendPatient encodes a canonical patient via the spec, sends it to the spec's
// destination over MLLP, persists it, and records the ACK result.
func (r *Router) SendPatient(ctx context.Context, p canonical.Patient) (int64, error) {
	ctrl := r.nextCtrl()
	raw, err := r.Spec.EncodeMessage(p, p.Event, ctrl)
	if err != nil {
		return 0, fmt.Errorf("encode message: %w", err)
	}
	msgType, _, _ := r.Spec.MessageTypeFor(p.Event)

	id, err := r.Store.Save(ctx, store.Message{
		Channel: r.Spec.Name, Direction: store.Outbound, Type: msgType,
		ControlID: ctrl, Raw: raw, Status: "sent",
	})
	if err != nil {
		return 0, err
	}

	ackRaw, err := mllp.Send(r.Spec.Transport.Address, []byte(raw), defaultSendTimeout)
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
	r.log().Info("sent message", "id", id, "type", msgType, "ack", code)
	return id, nil
}

// Sink receives patients decoded from inbound messages (e.g. the OpenEMR
// adapter webhook). Nil sink = store-and-ack only.
type Sink interface {
	WritePatient(ctx context.Context, p canonical.Patient) error
}

// InboundHandler decodes an inbound ADT, persists it, optionally writes it to a
// sink, and returns an ACK.
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
			Channel: r.Spec.Name, Direction: store.Inbound,
			Type: "ADT^" + dec.Trigger, ControlID: dec.ControlID, Raw: raw, Status: "received",
		})
		if serr != nil {
			r.log().Error("persist inbound", "err", serr)
		}

		ackCode, ackText := hl7.ACKAccept, ""
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
