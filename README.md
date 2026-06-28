# open-hl7

An open-source, EMR-agnostic **HL7 interface deployer** — a Mirth-Connect-style
channel engine designed to be deployed on-demand. OpenEMR is the first
integration; the hub itself knows nothing about any specific EMR.

## Architecture

```
EMR adapter (e.g. oe-module-open-hl7)            open-hl7 (this repo)
  • subscribes to EMR events          POST /events   • control-plane HTTP API
  • maps native model -> canonical  ───────────────▶  • canonical -> HL7 ADT
  • receives canonical via webhook  ◀───────────────  • MLLP client + server (framing + ACK)
                                                       • SQLite message store (audit/replay)
```

The hub speaks a **neutral canonical model** (`internal/canonical`). Each EMR
ships a thin adapter that maps its events to canonical and writes canonical
back; the hub never imports EMR-specific code. That is what makes "works with
any EMR" real — OpenEMR is just adapter #1.

## The differentiator: agentic connector builder

Instead of hand-reverse-engineering each system's interface, a vision-capable
**Claude agent reviews screenshots** of the target EMR (config screens, field
layouts, sample messages) and **synthesizes a connector on the fly** — a
declarative `ConnectorSpec` (transport + MSH routing + field map) that the hub
runs as a live channel, with a human approving before deploy. See
[docs/agentic-connector-builder.md](docs/agentic-connector-builder.md) and the
spec types in `internal/connectorgen`.

The OpenEMR adapter lives in this repo at `adapters/openemr/oe-module-hl7-hub`
(it installs into OpenEMR as a module — it is **not** committed to the OpenEMR
codebase).

## Milestone 1 — ADT patient feed (this scaffold)

- **Outbound:** `POST /events` with a canonical patient → encode `ADT^A04`
  (create) / `ADT^A08` (update) → MLLP send to the destination → store + ACK.
- **Inbound:** MLLP listener decodes `ADT` → canonical → store → ACK (write-back
  into an EMR via a `channel.Sink` is the next step).
- **Audit:** every message + ack state persisted to SQLite (`GET /messages`).

## Layout

```
cmd/hubd           daemon: MLLP listener + control-plane HTTP
cmd/testlistener   throwaway downstream MLLP endpoint that ACKs (for demos)
internal/canonical neutral clinical model (Patient)
internal/hl7       minimal HL7 v2 encode/decode + ACK (MSH/EVN/PID/MSA)
internal/mllp      MLLP framing: server + client
internal/store     SQLite message archive (Store interface; Postgres later)
internal/channel   routing: canonical <-> HL7 <-> MLLP, with persistence
internal/controlplane  HTTP API: /events, /messages, /health
```

## Run the end-to-end ADT loop

```bash
go build ./...

# terminal 1 — a fake downstream lab/EMR that ACKs everything
go run ./cmd/testlistener -listen :2576

# terminal 2 — the hub (outbound destination = the test listener)
go run ./cmd/hubd -mllp-listen :2575 -http :8088 -dest 127.0.0.1:2576 -db hub.db

# terminal 3 — push a canonical patient; hub emits ADT^A08 over MLLP
curl -s localhost:8088/events -H 'content-type: application/json' -d '{
  "mrn":"12345","familyName":"Doe","givenName":"Jane","birthDate":"19800115",
  "sex":"F","event":"create","source":{"emr":"openemr","facility":"CLINIC"}
}'

# inspect the archive
curl -s localhost:8088/messages | jq .
```

The test listener prints the received ADT; `GET /messages` shows it stored with
`ack_code: AA`.

## Roadmap

- **M2** — Lab ORM/ORU (reuse OpenEMR's `procedure_providers` + segment maps).
- **M3** — message-store dashboard + replay/retry queue.
- **M4** — on-demand deploy control-plane (container/tenant); Postgres store.
- **M5** — second EMR adapter (proves EMR-agnosticism).

## Status

Milestone-1 scaffold. Not production-ready: no TLS/auth on MLLP, no retry/queue,
ADT only, single channel from flags (YAML channel config is the next increment).
