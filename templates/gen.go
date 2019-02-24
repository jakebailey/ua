package templates

//go:generate qtc
//go:generate sh -c "sed -i 's://line:// line:g' *.qtpl.go"
//go:generate sh -c "gofmt -s -w *.qtpl.go"
