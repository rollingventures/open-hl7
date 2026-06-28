// Command testlistener is a throwaway downstream MLLP endpoint: it accepts
// framed HL7, prints it, and ACKs (AA). Use it to exercise the hub's outbound
// ADT path without a real lab/EMR on the other end.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"

	"github.com/rollingventures/open-hl7/internal/hl7"
	"github.com/rollingventures/open-hl7/internal/mllp"
)

func main() {
	addr := flag.String("listen", ":2576", "MLLP listen address")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	srv := &mllp.Server{
		Addr: *addr,
		Handler: func(_ context.Context, payload []byte, remote net.Addr) ([]byte, error) {
			fmt.Printf("---- received from %s ----\n%s\n", remote, payload)
			msg, err := hl7.Parse(string(payload))
			if err != nil {
				return nil, err
			}
			return []byte(hl7.BuildACK(msg.Segment("MSH"), hl7.ACKAccept, "").Encode()), nil
		},
	}
	if err := srv.ListenAndServe(ctx); err != nil {
		slog.Error("listener stopped", "err", err)
		os.Exit(1)
	}
}
