package app

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/gobwas/ws"
	"github.com/jakebailey/ua/models"
	"github.com/jakebailey/ua/pkg/ctxlog"
	"github.com/jakebailey/ua/pkg/docker/proxy"
	"github.com/jakebailey/ua/pkg/expire"
	"github.com/jakebailey/ua/templates"
	"go.uber.org/zap"
	"gopkg.in/src-d/go-kallax.v1"
)

func (a *App) routeInstance(r chi.Router) {
	r.Use(middleware.NoCache)

	r.Route("/{instanceID}", func(r chi.Router) {
		if a.debug {
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {
				wsURL := r.Host + r.RequestURI + "/ws"
				templates.WriteContainer(w, wsURL)
			})
		}

		r.Get("/ws", a.instanceWS)
	})
}

func (a *App) instanceWS(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := ctxlog.FromContext(ctx)

	instanceIDStr := chi.URLParam(r, "instanceID")
	instanceID, err := kallax.NewULIDFromText(instanceIDStr)
	if err != nil {
		logger.Warn("error parsing instanceID",
			zap.Error(err),
		)
		a.httpError(w, err.Error(), http.StatusBadRequest)
		return
	}

	instanceQuery := models.NewInstanceQuery().FindByID(instanceID).FindByActive(true)
	instance, err := a.instanceStore.FindOne(instanceQuery)
	if err != nil {
		if err == kallax.ErrNotFound {
			http.NotFound(w, r)
			return
		}

		logger.Error("error querying for instance",
			zap.Error(err),
			zap.String("instance_id", instanceIDStr),
		)
		a.httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	conn, _, _, err := ws.UpgradeHTTP(r, w, nil)
	if err != nil {
		// No need to write an error, UpgradeHTTP does this itself.
		logger.Error("error upgrading websocket",
			zap.Error(err),
		)
		return
	}

	proxyConn := proxy.NewWSConn(conn)

	if err := proxyConn.WriteJSON([]string{"stdout", "Please wait..."}); err != nil {
		logger.Warn("error writing please wait message",
			zap.Error(err),
		)
	}

	ctx = context.Background()
	ctx = ctxlog.WithLogger(ctx, logger)

	a.wsWG.Add(1)
	go a.handleInstance(ctx, proxyConn, instance)
}

func (a *App) handleInstance(ctx context.Context, conn proxy.Conn, instance *models.Instance) {
	defer a.wsWG.Done()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ctx, logger := ctxlog.FromContextWith(ctx,
		zap.String("instance_id", instance.ID.String()),
		zap.String("container_id", instance.ContainerID),
	)

	token := a.wsManager.Acquire(
		instance.ID.String(),
		func() {
			logger.Debug("websocket expired")
			if err := conn.Close(); err != nil {
				if strings.Contains(err.Error(), "use of closed connection") {
					return
				}

				logger.Error("error closing connection on expiry",
					zap.Error(err),
				)
			}
		},
	)
	defer a.wsManager.Release(token)

	instance.ExpiresAt = nil
	if _, err := a.instanceStore.Update(instance, models.Schema.Instance.ExpiresAt); err != nil {
		logger.Error("error disabling expiry for instance",
			zap.Error(err),
		)
	}

	if err := a.cli.ContainerStart(ctx, instance.ContainerID, types.ContainerStartOptions{}); err != nil {
		logger.Error("error starting container",
			zap.Error(err),
		)
	}

	conn = tokenProxyConn{
		Conn:  conn,
		token: token,
	}

	proxyCmd := proxy.Command(instance.Command)

	if err := proxy.Proxy(ctx, instance.ContainerID, conn, a.cli, proxyCmd); err != nil {
		logger.Error("error proxying container",
			zap.Error(err),
		)
	}

	instance.ExpiresAt = a.instanceExpireTime()

	if _, err := a.instanceStore.Update(instance, models.Schema.Instance.ExpiresAt); err != nil {
		logger.Error("error adding ExpiresAt to instance",
			zap.Error(err),
		)
	}

	second := time.Second
	if err := a.cli.ContainerStop(ctx, instance.ContainerID, &second); err != nil {
		logger.Error("error stopping container",
			zap.Error(err),
		)
	}
}

type tokenProxyConn struct {
	proxy.Conn
	token *expire.Token
}

var _ proxy.Conn = tokenProxyConn{}

func (t tokenProxyConn) ReadJSON(v interface{}) error {
	t.token.Update()
	return t.Conn.ReadJSON(v)
}

func (t tokenProxyConn) WriteJSON(v interface{}) error {
	t.token.Update()
	return t.Conn.WriteJSON(v)
}
