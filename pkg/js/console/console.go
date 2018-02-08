// Package console implements the global JS console object, in the same fashion
// as in the goja_node repo. However, it allows the setting of a stdout, per
// runtime. Since native modules are implemented as globals, users of this
// package must call Set and Cleanup to manage their outputs, and prevent
// leaks.
package console

import (
	"io"
	"sync"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
	_ "github.com/dop251/goja_nodejs/util" // For require('util').format().
)

var (
	consoleWriters sync.Map
)

// Set sets the provided runtime's stdout to the given io.Writer.
// By default, no stdout is set, and any logging will be ignored.
// For goja_nodejs like behavior, set the writer to os.Stdout.
//
// When finished with a runtime, be sure to call Cleanup.
func Set(runtime *goja.Runtime, w io.Writer) {
	consoleWriters.Store(runtime, w)
}

// Cleanup unsets the runtime's console output. If an output is set, and
// Cleanup is not called, the runtime will be leaked (as this package will)
// still have a reference to it.
func Cleanup(runtime *goja.Runtime) {
	consoleWriters.Delete(runtime)
}

type console struct {
	runtime *goja.Runtime
	util    *goja.Object
}

func (c *console) log(call goja.FunctionCall) goja.Value {
	v, ok := consoleWriters.Load(c.runtime)
	if !ok {
		return nil
	}

	w, ok := v.(io.Writer)
	if !ok || w == nil {
		return nil
	}

	format, ok := goja.AssertFunction(c.util.Get("format"))
	if !ok {
		panic(c.runtime.NewTypeError("util.format is not a function"))
	}

	ret, err := format(c.util, call.Arguments...)
	if err != nil {
		panic(err)
	}

	if _, err = w.Write([]byte(ret.String())); err != nil {
		panic(c.runtime.NewGoError(err))
	}

	return nil
}

// Require builds the console module using the given runtime
// and module object. This function signature is the same as ModuleLoader.
func Require(runtime *goja.Runtime, module *goja.Object) {
	c := &console{
		runtime: runtime,
	}

	c.util = require.Require(runtime, "util").(*goja.Object)

	o := module.Get("exports").(*goja.Object)
	o.Set("log", c.log)
	o.Set("error", c.log)
	o.Set("warn", c.log)
}

// Enable sets the global console object to require('console').
func Enable(runtime *goja.Runtime) {
	runtime.Set("console", require.Require(runtime, "console"))
}

func init() {
	require.RegisterNativeModule("console", Require)
}
