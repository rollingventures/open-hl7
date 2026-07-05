package transform

//go:generate sh build_guest.sh

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// defaultWasm is the sample transform guest (see examples/wasm-transform/guest).
//
//go:embed assets/transform.wasm
var defaultWasm []byte

// DefaultWasm returns the embedded sample transform module.
func DefaultWasm() []byte { return defaultWasm }

// WasmTransformer runs a WASM transform module via wazero. It compiles the
// module once and instantiates a fresh, isolated instance per call — the guest
// gets no filesystem or network (only WASI clock/random), and a cancelled
// context aborts execution.
type WasmTransformer struct {
	runtime wazero.Runtime
	code    wazero.CompiledModule
}

// NewWasmTransformer compiles a WASM transform. Pass nil to use the embedded
// sample module. Call Close when done.
func NewWasmTransformer(ctx context.Context, wasm []byte) (*WasmTransformer, error) {
	if len(wasm) == 0 {
		wasm = defaultWasm
	}
	rt := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().WithCloseOnContextDone(true))
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, rt); err != nil {
		_ = rt.Close(ctx)
		return nil, fmt.Errorf("transform: wasi: %w", err)
	}
	code, err := rt.CompileModule(ctx, wasm)
	if err != nil {
		_ = rt.Close(ctx)
		return nil, fmt.Errorf("transform: compile: %w", err)
	}
	return &WasmTransformer{runtime: rt, code: code}, nil
}

// Close releases the runtime.
func (w *WasmTransformer) Close(ctx context.Context) error {
	return w.runtime.Close(ctx)
}

// Transform runs the module against the given canonical JSON input.
func (w *WasmTransformer) Transform(ctx context.Context, input []byte) ([]byte, error) {
	// No FS, no sockets configured -> the guest cannot reach the host.
	mod, err := w.runtime.InstantiateModule(ctx, w.code,
		wazero.NewModuleConfig().WithName("").WithStartFunctions())
	if err != nil {
		return nil, fmt.Errorf("transform: instantiate: %w", err)
	}
	defer mod.Close(ctx)

	// Reactor modules (Go -buildmode=c-shared) run package init via _initialize.
	if initFn := mod.ExportedFunction("_initialize"); initFn != nil {
		if _, err := initFn.Call(ctx); err != nil {
			return nil, fmt.Errorf("transform: init: %w", err)
		}
	}

	inPtr, err := call1(ctx, mod, "input_ptr")
	if err != nil {
		return nil, err
	}
	if !mod.Memory().Write(uint32(inPtr), input) {
		return nil, fmt.Errorf("transform: input of %d bytes exceeds guest buffer", len(input))
	}

	outLen, err := call1(ctx, mod, "transform", uint64(len(input)))
	if err != nil {
		return nil, err
	}
	if int32(outLen) < 0 {
		return nil, fmt.Errorf("transform: guest reported an error")
	}

	outPtr, err := call1(ctx, mod, "output_ptr")
	if err != nil {
		return nil, err
	}
	data, ok := mod.Memory().Read(uint32(outPtr), uint32(int32(outLen)))
	if !ok {
		return nil, fmt.Errorf("transform: output range out of bounds")
	}
	out := make([]byte, len(data))
	copy(out, data)
	return out, nil
}

func call1(ctx context.Context, mod api.Module, name string, args ...uint64) (uint64, error) {
	fn := mod.ExportedFunction(name)
	if fn == nil {
		return 0, fmt.Errorf("transform: guest missing export %q", name)
	}
	res, err := fn.Call(ctx, args...)
	if err != nil {
		return 0, fmt.Errorf("transform: calling %s: %w", name, err)
	}
	if len(res) == 0 {
		return 0, nil
	}
	return res[0], nil
}
