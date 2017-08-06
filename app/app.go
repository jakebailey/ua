package app

import (
	"sync"

	"github.com/docker/docker/client"
	"github.com/go-chi/chi"
	"github.com/satori/go.uuid"
)

type App struct {
	debug          bool
	assignmentPath string

	cli client.CommonAPIClient

	active   map[uuid.UUID]string
	activeMu sync.RWMutex
}

func NewApp(cli client.CommonAPIClient, options ...Option) *App {
	a := &App{
		assignmentPath: "assignments",
		cli:            cli,
		active:         make(map[uuid.UUID]string),
	}

	for _, o := range options {
		o(a)
	}

	return a
}

func (a *App) Route(r chi.Router) {
	r.Route("/assignments", a.routeAssignments)
	r.Route("/containers", a.routeContainers)
}

type Option func(*App)

func Debug() Option {
	return func(a *App) {
		a.debug = true
	}
}

func AssignmentPath(path string) Option {
	return func(a *App) {
		a.assignmentPath = path
	}
}
