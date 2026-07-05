// Separate module so the host build (go build ./...) never tries to compile
// this wasip1-only guest. Built to WASM by internal/transform/build_guest.sh.
module openhl7.example/wasm-transform-guest

go 1.25
