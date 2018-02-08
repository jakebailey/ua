package jslib

//go:generate esc -o fs.go -pkg jslib -ignore=\.go$ -private .

var modulePaths = map[string]string{
	"lodash": "/lodash.js",
}

func Load(name string) ([]byte, error) {
	return _escFSByte(false, modulePaths[name])
}
