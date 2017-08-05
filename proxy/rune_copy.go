package proxy

import (
	"bufio"
	"bytes"
	"io"
	"log"
)

// RuneCopy is like io.Copy, but only writes valid runes, waiting for more input if invalid.
// This uses ScanRunesGreedy to write as much as possible per read.
func RuneCopy(dst io.Writer, src io.Reader) (written int, err error) {
	var n int
	s := bufio.NewScanner(src)
	s.Split(ScanRunesGreedy)

	for s.Scan() {
		if s.Err() != nil {
			log.Printf("scanner error: %s", err)
			break
		}
		n, err = dst.Write(s.Bytes())
		written += n
		if err != nil {
			return
		}
	}

	return
}

// ScanRunesGreedy is like bufio.ScanRunes, but will capture as many runes from the input as possible.
func ScanRunesGreedy(data []byte, atEOF bool) (advance int, token []byte, err error) {
	buf := &bytes.Buffer{}
	var a int
	var t []byte

	for advance < len(data) {
		a, t, err = bufio.ScanRunes(data[advance:], atEOF)
		advance += a
		if _, writeErr := buf.Write(t); err != nil {
			panic(writeErr) // Write() always succeeds, unless it panics with ErrTooLarge (out of memory)
		}

		if err != nil {
			break
		}
		if a == 0 {
			break
		}
	}

	return advance, buf.Bytes(), err
}
