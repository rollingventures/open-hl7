# Project status

Snapshot of where open-hl7 stands, so work can resume cleanly.

## Shipped / working

- **Repo:** `github.com/rollingventures/open-hl7` (public). `v0.1.0` released
  (GoReleaser: linux/darwin × amd64/arm64 + checksums). `curl … | sh` installer
  verified live. Website: https://rollingventures.github.io/open-hl7/.
- **Hub (M1 ADT):** MLLP server + client with framing/ACK, HL7 v2 encode/decode
  (MSH/EVN/PID/MSA, ADT^A04/A08), SQLite message archive (`Store` interface),
  channel router, control-plane API (`POST /events`, `GET /messages`, `/health`).
  Verified end-to-end.
- **CB-1 — spec-driven engine:** `internal/connectorgen` — declarative
  `ConnectorSpec` (transport + MSH routing + canonical⇄HL7 field map),
  `EncodeMessage`, `LoadSpec`, `OpenEMRADTSpec`. `hubd -spec <json>` runs any
  spec. Example: `configs/openemr-adt.connector.json`.
- **WASM transforms:** `internal/transform` (wazero, pure-Go) runs sandboxed
  transforms (no FS/net) as an alternative to the field map. `hubd
  -wasm-transform embedded|<path>`. Sample guest in `examples/wasm-transform/`.
  See `docs/wasm-transforms.md`.
- **OpenEMR adapter:** `adapters/openemr/oe-module-hl7-hub` — thin module,
  canonical-model bridge (not HL7). Unit + adapter→hub integration tests; CI job
  `adapter-openemr`. **Not yet verified against a running OpenEMR stack.**
- **CI:** `ci.yml` (Go build/vet/test + PHP adapter tests), `release.yml` (tag
  `v*`), `pages.yml`.

## Architecture rule

The hub never imports EMR-specific code; it speaks a neutral canonical model.
Each EMR is a thin adapter (only its mapper knows the EMR) plus a
`ConnectorSpec`. OpenEMR is adapter #1. The OpenEMR adapter installs *into*
OpenEMR but is **not** committed to the OpenEMR codebase.

## Decisions

- Runtime: Go. Store: SQLite (Postgres at M4). First flow: ADT.
- Agent runtime (for CB-2): **Claude Agent SDK sidecar (TS/Python)**.

## Next up

- **CB-2 — agentic connector builder:** the sidecar that reads target-EMR
  screenshots/sample messages and emits a `ConnectorSpec` (and/or a WASM
  transform) via the Go `connectorgen.Builder` seam. `PERCEIVE → MAP`, then
  CB-3 `VALIDATE` (generate a message, diff vs the real sample, self-correct).
  Redact PHI/credentials from images before any API call.
- Verify the OpenEMR adapter against a live stack; wire the inbound
  `channel.Sink` for write-back.
- M2: lab ORM/ORU; M3: store dashboard + replay; M4: on-demand deploy + Postgres.

## Regenerating the sample WASM

```sh
sh internal/transform/build_guest.sh   # Go 1.24+ (//go:wasmexport)
```
