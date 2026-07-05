// Command hubd is the HL7 hub daemon: an MLLP listener (inbound HL7 -> store +
// ACK) and a control-plane HTTP API (canonical events -> outbound HL7 over
// MLLP). Outbound encoding is driven by a ConnectorSpec — supply one with
// -spec, or omit it to use the built-in OpenEMR ADT reference spec.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/rollingventures/open-hl7/internal/channel"
	"github.com/rollingventures/open-hl7/internal/connectorgen"
	"github.com/rollingventures/open-hl7/internal/controlplane"
	"github.com/rollingventures/open-hl7/internal/mllp"
	"github.com/rollingventures/open-hl7/internal/store"
	"github.com/rollingventures/open-hl7/internal/transform"
)

// version is stamped at release time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	var (
		showVersion = flag.Bool("version", false, "print version and exit")
		mllpAddr = flag.String("mllp-listen", ":2575", "MLLP listen address (inbound HL7)")
		httpAddr = flag.String("http", ":8088", "control-plane HTTP listen address")
		dbPath   = flag.String("db", "hub.db", "SQLite database path")
		dest     = flag.String("dest", "127.0.0.1:2576", "destination MLLP address (used by the default spec)")
		secret   = flag.String("secret", os.Getenv("HUB_SECRET"), "shared secret required on POST /events")
		chanName = flag.String("channel", "openemr-adt", "channel name (used by the default spec)")
		specPath = flag.String("spec", "", "path to a ConnectorSpec JSON file (default: built-in OpenEMR ADT spec)")
		wasmPath = flag.String("wasm-transform", "", `WASM transform module: a path to a .wasm, or "embedded" for the built-in sample. Overrides the field map for outbound encoding.`)
	)
	flag.Parse()

	if *showVersion {
		println("open-hl7 " + version)
		return
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
	logger.Info("open-hl7 starting", "version", version)

	spec := connectorgen.OpenEMRADTSpec(*chanName, *dest)
	if *specPath != "" {
		loaded, err := connectorgen.LoadSpec(*specPath)
		if err != nil {
			logger.Error("load connector spec", "err", err)
			os.Exit(1)
		}
		spec = loaded
		logger.Info("loaded connector spec", "name", spec.Name, "system", spec.System, "path", *specPath)
	}

	st, err := store.OpenSQLite(*dbPath)
	if err != nil {
		logger.Error("open store", "err", err)
		os.Exit(1)
	}
	defer st.Close()

	router := &channel.Router{Spec: spec, Store: st, Logger: logger}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *wasmPath != "" {
		var wasmBytes []byte
		if *wasmPath != "embedded" {
			wasmBytes, err = os.ReadFile(*wasmPath)
			if err != nil {
				logger.Error("read wasm transform", "err", err)
				os.Exit(1)
			}
		}
		wt, werr := transform.NewWasmTransformer(ctx, wasmBytes)
		if werr != nil {
			logger.Error("init wasm transform", "err", werr)
			os.Exit(1)
		}
		defer wt.Close(ctx)
		router.Transformer = wt
		logger.Info("WASM transform enabled", "source", *wasmPath)
	}

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
