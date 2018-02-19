package js

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
	"github.com/jakebailey/ua/pkg/js/console"
	"github.com/jakebailey/ua/pkg/js/jslib"
)

// Runtime wraps a goja runtime.
type Runtime struct {
	vm     *goja.Runtime
	stdout io.Writer
	loader func(name string) ([]byte, error)
}

// Options is provided to NewRuntime to construct a new Runtime.
type Options struct {
	// Stdout is where the console writes.
	// If nil, then the console writes nowhere.
	Stdout io.Writer

	// ModuleLoader is a function which loads JS source by name.
	// If nil, then then no modules can be loaded (other than embedded ones,
	// see DisableLibs).
	ModuleLoader func(name string) ([]byte, error)

	// FileReader is a function which reads a file into the runtime. If not nil,
	// then this function is accessible through the name "readFile".
	FileReader func(filename string) ([]byte, error)

	// DisableLibs controls access to embedded libraries (lodash, etc).
	// If false, then they will not be accessible.
	DisableLibs bool
}

// NewRuntime creates a new js runtime. Once the runtime is no longer needed,
// call Destroy.
func NewRuntime(options Options) *Runtime {
	r := &Runtime{
		vm: goja.New(),
	}

	r.stdout = options.Stdout

	if options.FileReader != nil {
		r.vm.Set("readFile", func(call goja.FunctionCall) goja.Value {
			filename := call.Argument(0).String()
			contents, err := options.FileReader(filename)
			if err != nil {
				panic(r.vm.NewGoError(err))
			}
			return r.vm.ToValue(string(contents))
		})
	}

	moduleLoader := options.ModuleLoader

	switch {
	case options.DisableLibs:
		r.loader = moduleLoader
	case moduleLoader == nil:
		r.loader = jslib.Load
	default:
		r.loader = func(moduleName string) ([]byte, error) {
			b, err := jslib.Load(moduleName)
			if err == nil {
				return b, err
			}
			return moduleLoader(moduleName)
		}
	}

	registry := require.NewRegistryWithLoader(r.loader)
	registry.Enable(r.vm)
	console.Enable(r.vm)

	if r.stdout != nil {
		console.Set(r.vm, r.stdout)
	}

	r.vm.Set("btoa", r.btoa)
	r.vm.Set("atob", r.atob)

	return r
}

// Destroy cleans up the runtime. After calling Destrory, the runtime
// may not be usable.
func (r *Runtime) Destroy() {
	if r.stdout != nil {
		console.Cleanup(r.vm)
	}
}

// Run runs a program in the runtime, and exports the result to out via JSON.
// If the context is cancelled, then the runtime is interrupted, and the output
// is undefined.
func (r *Runtime) Run(ctx context.Context, program string, out interface{}) error {
	stop := make(chan struct{})
	defer func() {
		stop <- struct{}{}
		close(stop)
	}()

	go func() {
		select {
		case <-stop:
			// Normal return. Just return without interrupting the VM.
		case <-ctx.Done():
			// Interrupt the VM with the context's error, likely either
			// context.Canceled or context.DeadlineExceeded.
			r.vm.Interrupt(ctx.Err())
		}
	}() // Exits when Run returns, or when the context is cancelled.

	v, err := r.vm.RunString(program)
	if err != nil {
		// If the error is due to an interrupt, then attempt to extract
		// the InterruptedError's value as an error and return it instead
		// (i.e., grab a potential ctx.Err from above).
		if iErr, ok := err.(*goja.InterruptedError); ok {
			if iErr, ok := iErr.Value().(error); ok {
				err = iErr
			}
		}
		return err
	}
	return r.exportViaJSON(v, out)
}

// Workaround, since ExportTo is case sensitive.
func (r *Runtime) exportViaJSON(val goja.Value, out interface{}) error {
	if out == nil {
		return nil
	}

	gojaJSON, ok := r.vm.Get("JSON").(*goja.Object)
	if !ok {
		return errors.New("missing JSON object")
	}

	stringify, ok := goja.AssertFunction(gojaJSON.Get("stringify"))
	if !ok {
		return errors.New("missing JSON.stringify object")
	}

	res, err := stringify(gojaJSON, val)
	if err != nil {
		return err
	}

	buf := []byte(res.String())
	return json.Unmarshal(buf, out)
}

// Set sets a variable to the specified value in the runtime.
func (r *Runtime) Set(name string, value interface{}) {
	r.vm.Set(name, value)
}

func (r *Runtime) btoa(call goja.FunctionCall) goja.Value {
	input := call.Argument(0).String()
	return r.vm.ToValue(base64.StdEncoding.EncodeToString([]byte(input)))
}

func (r *Runtime) atob(call goja.FunctionCall) goja.Value {
	input := call.Argument(0).String()
	buf, err := base64.StdEncoding.DecodeString(input)
	if err != nil {
		panic(r.vm.NewGoError(err))
	}
	return r.vm.ToValue(string(buf))
}
