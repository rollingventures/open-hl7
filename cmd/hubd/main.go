// Command hubd is the HL7 hub daemon: an MLLP listener (inbound HL7 -> store +
// ACK) and a control-plane HTTP API (canonical events -> outbound ADT over
// MLLP). M1 scope: the ADT patient feed, end to end.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rollingventures/hl7-hub/internal/channel"
	"github.com/rollingventures/hl7-hub/internal/controlplane"
	"github.com/rollingventures/hl7-hub/internal/mllp"
	"github.com/rollingventures/hl7-hub/internal/store"
)

func main() {
	var (
		mllpAddr = flag.String("mllp-listen", ":2575", "MLLP listen address (inbound HL7)")
		httpAddr = flag.String("http", ":8088", "control-plane HTTP listen address")
		dbPath   = flag.String("db", "hub.db", "SQLite database path")
		dest     = flag.String("dest", "127.0.0.1:2576", "destination MLLP address for outbound ADT")
		secret   = flag.String("secret", os.Getenv("HUB_SECRET"), "shared secret required on POST /events")
		chanName = flag.String("channel", "openemr-adt", "channel name")
	)
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	st, err := store.OpenSQLite(*dbPath)
	if err != nil {
		logger.Error("open store", "err", err)
		os.Exit(1)
	}
	defer st.Close()

	router := &channel.Router{
		Cfg: channel.Config{
			Name:            *chanName,
			DestinationAddr: *dest,
			SendingApp:      "OPENEMR",
			SendingFac:      "OPENEMR",
			ReceivingApp:    "HUB",
			ReceivingFac:    "HUB",
			SendTimeout:     10 * time.Second,
		},
		Store:  st,
		Logger: logger,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Inbound MLLP server: decode ADT -> store -> ACK. Sink is nil for M1
	// (store-and-ack); wiring an OpenEMR write-back sink is the next step.
	mllpSrv := &mllp.Server{
		Addr:    *mllpAddr,
		Handler: router.InboundHandler(nil),
		Logger:  logger,
	}
	go func() {
		if err := mllpSrv.ListenAndServe(ctx); err != nil {
			logger.Error("mllp server stopped", "err", err)
			stop()
		}
	}()

	// Control plane HTTP.
	cp := &controlplane.Server{Addr: *httpAddr, Router: router, Store: st, Logger: logger, Secret: *secret}
	if err := cp.ListenAndServe(ctx); err != nil {
		logger.Error("control plane stopped", "err", err)
		os.Exit(1)
	}

	logger.Info("hubd shut down cleanly")
}
