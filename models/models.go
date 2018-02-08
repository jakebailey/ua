package models

import (
	"time"

	"gopkg.in/src-d/go-kallax.v1"
)

// Spec specifies how to build an assignment's image from scratch.
// This includes the assignment's name (same as the subdirectory),
// data and seed to pass to the template, and any instances associated
// with the spec.
type Spec struct {
	kallax.Model `table:"specs"`
	kallax.Timestamps
	ID             kallax.ULID `pk:""`
	AssignmentName string
	Data           interface{}
	Instances      []*Instance
}

func newSpec() *Spec {
	return &Spec{
		ID: kallax.NewULID(),
	}
}

// Instance describes a single instance of a Spec. This includes
// the ID of the Docker image, the ID of the Docker container,
// when the instance should expire (a new instance must be created),
// and its status.
type Instance struct {
	kallax.Model `table:"instances"`
	kallax.Timestamps
	ID          kallax.ULID `pk:""`
	Spec        *Spec       `fk:",inverse"`
	ImageID     string
	ContainerID string
	ExpiresAt   *time.Time
	Active      bool
	Cleaned     bool
	Command     InstanceCommand
}

func newInstance() *Instance {
	return &Instance{
		ID: kallax.NewULID(),
	}
}

// InstanceCommand is the command that should be proxied to the user
// of an instance.
type InstanceCommand struct {
	User       string
	Cmd        []string
	Env        []string
	WorkingDir string
}

//go:generate kallax gen
