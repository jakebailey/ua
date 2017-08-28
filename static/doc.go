// Package static contains the static HTTP resources served at /static/.
package static

//go:generate esc -o=static.go -pkg=static -ignore=^(doc|static)\.go$ .
