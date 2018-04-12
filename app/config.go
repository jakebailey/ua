package app

import (
	"errors"
	"time"
)

// Config is a set of human-readable configuration for the app.
type Config struct {
	// Addr is the address where the app will run its HTTP/S server.
	Addr string

	// LetsEncryptDomain is the domain to obtain LetsEncrypt certs for.
	LetsEncryptDomain string
	// CertFile is a path to a certificate file for HTTPS.
	CertFile string
	// KeyFile is a path to a key file for HTTPS.
	KeyFile string

	// Database is the PostgreSQL database connection string.
	Database string
	// MigrateUp enables upward database migration at startup.
	MigrateUp bool
	// MigrateReset enables upward database migration via database reset at startup.
	MigrateReset bool

	// AssignmentPath is the path assignments are stored in. If relative,
	// then this will be relative to the current working directory.
	AssignmentPath string
	// StaticPath is the path to the static elements served at /static
	// by the app.
	StaticPath string

	// AESKey is a base64-encoded string containing the AES key.
	AESKey string

	// CleanInactiveEvery is the period at which the app will clean up
	// inactive images and containers.
	CleanInactiveEvery time.Duration
	// CheckExpiredEvery is the period at which the app will check for
	// active instances past their expiry time and stop them.
	CheckExpiredEvery time.Duration
	// WebsocketTimeout is the maximum duration a websocket can be
	// inactive before expiring.
	WebsocketTimeout time.Duration
	// InstanceExpire is the maximum duration an instance will be kept
	// on the server until it expires and a new instance must be created.
	InstanceExpire time.Duration
	// ForceInactive enables forced instance inactive marking at startup/shutdown.
	ForceInactive bool

	// DisableLimits disables Docker container limits.
	DisableLimits bool

	// DisableAutoPull disables automatic image pulling (for updates).
	DisableAutoPull bool
	// AutoPullEvery is the interval at which the server will attempt
	// to pull images that have been recently used (to keep them updated).
	AutoPullEvery time.Duration
	// AutoPullExpiry defines what the autopuller defines as "recent".
	AutoPullExpiry time.Duration

	// PruneEvery is the interval at which the server will prune docker.
	PruneEvery time.Duration

	// Debug enables debug routes.
	Debug bool

	// PProfToken is the token/password used for HTTP pprof connections.
	PProfToken string
}

// DefaultConfig is the App's default configuration.
var DefaultConfig = Config{
	Addr: ":8000",

	AssignmentPath: "assignments",
	StaticPath:     "static",

	CleanInactiveEvery: time.Hour,
	CheckExpiredEvery:  time.Minute,
	WebsocketTimeout:   time.Hour,
	InstanceExpire:     4 * time.Hour,

	AutoPullEvery:  time.Hour,
	AutoPullExpiry: 30 * time.Minute,

	PruneEvery: time.Hour,
}

// Verify verifies that the configuration is valid and usable.
func (c Config) Verify() error {
	if c.Database == "" {
		return errors.New("database connection string cannot be empty")
	}

	if c.MigrateUp && c.MigrateReset {
		return errors.New("both MigrateUp and MigrateReset cannot be set")
	}

	switch {
	case c.LetsEncryptDomain != "":
		if c.CertFile != "" || c.KeyFile != "" {
			return errors.New("cannot use both Let's Encrypt and regular TLS certs at the same time")
		}
	case c.CertFile != "" && c.KeyFile == "", c.CertFile == "" && c.KeyFile != "":
		return errors.New("both CertFile and KeyFile must be specified together")
	}

	return nil
}
