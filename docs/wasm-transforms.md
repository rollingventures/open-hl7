# WASM connector transforms

The declarative field map (`internal/connectorgen`, `SEG-field.component` paths)
covers most connectors. For logic a field map can't express — conditional
routing, custom code-system lookups, odd formatting, per-site quirks — a
connector can carry a **WASM transform**: guest code that the hub runs in a
sandbox.

## Why WASM

- **Safe to run untrusted / agent-generated code.** The agentic connector
  builder can emit a transform module; the hub runs it with **no filesystem and
  no network** (only WASI clock/random), and a cancelled context aborts it.
- **Polyglot.** Write transforms in Go, Rust, TinyGo, or AssemblyScript — any
  language that compiles to `wasm`.
- **Hot-loadable.** Ship a connector as a `.wasm`; no hub rebuild.

Network I/O stays on the host (MLLP/SFTP): today's WASI socket support is too
immature for a pure-WASM forwarder, so the split is **host owns transport, WASM
owns the transform.**

## Runtime

`internal/transform` embeds [wazero](https://wazero.io) — a pure-Go, no-cgo WASM
runtime. `WasmTransformer` compiles the module once and instantiates a fresh,
isolated instance per call.

```go
w, _ := transform.NewWasmTransformer(ctx, nil) // nil = embedded sample module
defer w.Close(ctx)
hl7Bytes, _ := w.Transform(ctx, canonicalJSON)
```

Wire it into a channel by setting `Router.Transformer`; when set it replaces the
field-map encoding. From the CLI:

```sh
open-hl7 -wasm-transform embedded   # use the built-in sample
open-hl7 -wasm-transform ./my.wasm  # use your own module
```

## ABI

The guest exposes three exports and uses two static buffers (which keeps memory
safe with Go's GC):

| export | meaning |
|--------|---------|
| `input_ptr() i32`  | pointer to the input buffer (host writes canonical JSON here) |
| `output_ptr() i32` | pointer to the output buffer (host reads the HL7 message here) |
| `transform(inLen i32) i32` | encode; returns output length, or `-1` on error |

Input JSON is the canonical patient plus `controlId` and `timestamp`.

## Sample guest

`examples/wasm-transform/guest` is a Go guest (a separate module so it never
enters the host build). It builds an ADT message and applies one rule a field
map can't — upper-casing the family name — to show arbitrary logic runs in the
sandbox. Rebuild the embedded `internal/transform/assets/transform.wasm` with:

```sh
sh internal/transform/build_guest.sh   # or: go generate ./internal/transform/
```

Requires Go 1.24+ (for `//go:wasmexport` reactor modules).
