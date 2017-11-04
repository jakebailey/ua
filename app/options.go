package app

import (
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/docker/docker/client"
	"go.uber.org/zap"
)

// Option is a function that runs on an App to configure it.
type Option func(*App)

// Debug sets the debug mode. The default is false.
func Debug(debug bool) Option {
	return func(a *App) {
		a.debug = debug
	}
}

// Addr sets the address of the app's HTTP server. If not provided,
// DefaultAddr is used.
func Addr(addr string) Option {
	return func(a *App) {
		a.addr = addr
	}
}

// AssignmentPath sets the path for the assignment directory. If not
// provided, DefaultAssignmentPath is used.
func AssignmentPath(path string) Option {
	return func(a *App) {
		a.assignmentPath = path
	}
}

// Logger sets the logger used within the app. If not provided,
// DefaultLogger is used.
func Logger(logger *zap.Logger) Option {
	return func(a *App) {
		a.logger = logger
	}
}

// SpewConfig sets the spew config state used for various debugging
// endpoints in the app. If not provided, DefaultSpew is used.
func SpewConfig(c *spew.ConfigState) Option {
	if c == nil {
		panic("app: spew ConfigState cannot be nil")
	}

	return func(a *App) {
		a.spew = c
	}
}

// DockerClient sets the docker client used in the app. If closeFunc is not
// nil, then it will be called when the app closes.
func DockerClient(cli client.CommonAPIClient, closeFunc func() error) Option {
	return func(a *App) {
		a.cli = cli
		a.cliClose = closeFunc
	}
}

// TLS enables TLS for the app's HTTP server. Using this option disables
// Let's encrypt.
func TLS(certFile, certKey string) Option {
	return func(a *App) {
		a.tls = true
		a.tlsCertFile = certFile
		a.tlsKeyFile = certKey

		a.letsEncrypt = false
		a.letsEncryptDomain = ""
	}
}

// LetsEncryptDomain set the domain used by Let's Encrypt. Using this option
// disables TLS by certfile/keyfile.
func LetsEncryptDomain(domain string) Option {
	return func(a *App) {
		a.letsEncrypt = true
		a.letsEncryptDomain = domain

		a.tls = false
		a.tlsCertFile = ""
		a.tlsKeyFile = ""
	}
}

// AESKey specifies the AES key used for encryption. AESKey will panic if the
// key's length is not 12, 24, or 32.
func AESKey(key []byte) Option {
	switch len(key) {
	case 16, 24, 32:
	default:
		panic("AES key must be of length 16, 24, or 32")
	}

	return func(a *App) {
		a.aesKey = key
	}
}

func CleanInactiveEvery(d time.Duration) Option {
	return func(a *App) {
		a.cleanInactiveEvery = d
	}
}

func CheckExpiredEvery(d time.Duration) Option {
	return func(a *App) {
		a.checkExpiredEvery = d
	}
}

func WebsocketTimeout(d time.Duration) Option {
	return func(a *App) {
		a.wsTimeout = d
	}
}

func InstanceExpire(d time.Duration) Option {
	return func(a *App) {
		a.instanceExpire = d
	}
}

func StaticPath(path string) Option {
	return func(a *App) {
		a.staticPath = path
	}
}

func MigrateUp(migrateUp bool) Option {
	return func(a *App) {
		a.migrateUp = migrateUp
	}
}

func MigrateReset(migrateReset bool) Option {
	return func(a *App) {
		a.migrateReset = migrateReset
	}
}

func DisableLimits(disableLimits bool) Option {
	return func(a *App) {
		a.disableLimits = disableLimits
	}
}

func PProfToken(token string) Option {
	return func(a *App) {
		a.pprofToken = token
	}
}
