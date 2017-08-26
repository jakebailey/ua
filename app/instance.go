package app

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"github.com/gobwas/ws"
	"github.com/jakebailey/ua/ctxlog"
	"github.com/jakebailey/ua/expire"
	"github.com/jakebailey/ua/models"
	"github.com/jakebailey/ua/proxy"
	"github.com/jakebailey/ua/templates"
	"go.uber.org/zap"
	"gopkg.in/src-d/go-kallax.v1"
)

func (a *App) routeInstance(r chi.Router) {
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

	instanceQuery := models.NewInstanceQuery().FindByID(instanceID)
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

	ctx = context.Background()
	ctx = ctxlog.WithLogger(ctx, logger)

	a.wsWG.Add(1)
	go a.handleInstance(ctx, conn, instance)
}

func (a *App) handleInstance(ctx context.Context, conn net.Conn, instance *models.Instance) {
	defer a.wsWG.Done()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ctx, logger := ctxlog.FromContextWith(ctx,
		zap.String("instance_id", instance.ID.String()),
		zap.String("container_id", instance.ContainerID),
	)

	var proxyConn proxy.Conn = proxy.NewWSConn(conn)

	token := a.wsManager.Acquire(
		instance.ID.String(),
		func() {
			logger.Debug("websocket expired")
			if err := proxyConn.Close(); err != nil {
				logger.Error("error closing connection on expiry",
					zap.Error(err),
				)
			}
		},
	)
	defer a.wsManager.Return(token)

	proxyConn = tokenProxyConn{
		Conn:  proxyConn,
		token: token,
	}

	if err := proxy.Proxy(ctx, instance.ContainerID, proxyConn, a.cli); err != nil {
		logger.Error("error proxying container",
			zap.Error(err),
		)
	}

	expiresAt := time.Now().Add(4 * time.Hour)
	instance.ExpiresAt = &expiresAt

	if _, err := a.instanceStore.Update(instance, models.Schema.Instance.ExpiresAt); err != nil {
		logger.Error("error adding ExpiresAt to instance",
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
