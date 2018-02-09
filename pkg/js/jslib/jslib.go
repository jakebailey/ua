package jslib

//go:generate esc -o fs.go -pkg jslib -ignore=\.go$ -private .

var modulePaths = map[string]string{
	"lodash": "/lodash.js",
}

// Load returns the source code for a given library by name, or an error.
func Load(name string) ([]byte, error) {
	return _escFSByte(false, modulePaths[name])
}
