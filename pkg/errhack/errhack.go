package errhack

import (
	"io"
	"strings"
)

// IsClose returns true if the error looks like a "close" error. Arguably,
// in well-contructed code, these should never appear, but it's hard to manage.
func IsClose(err error) bool {
	if err == io.EOF {
		return true
	}

	errText := err.Error()

	if strings.Contains(errText, "use of closed network connection") {
		return true
	}

	if strings.Contains(errText, "broken pipe") {
		return true
	}

	return false
}

// IgnoreClose returns nil if the provided error satisfies IsClose, otherwise
// it returns the provided error.
func IgnoreClose(err error) error {
	if IsClose(err) {
		return nil
	}
	return err
}
