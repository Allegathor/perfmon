package monserv

import (
	"context"
	"net"
	"net/http"

	"github.com/Allegathor/perfmon/internal/monserv/handlers"
	"github.com/Allegathor/perfmon/internal/monserv/middlewares"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type MonServ struct {
	*http.Server
	db     handlers.MDB
	Router *chi.Mux
	Logger *zap.SugaredLogger
}

func NewInstance(ctx context.Context, addr string, db handlers.MDB, l *zap.SugaredLogger) *MonServ {
	s := &MonServ{
		db:     db,
		Router: chi.NewRouter(),
		Logger: l,
	}

	s.Server = &http.Server{Addr: addr, BaseContext: func(l net.Listener) context.Context {
		return ctx
	}}

	return s
}

func (s *MonServ) MountHandlers() {
	api := handlers.NewAPI(s.db, s.Logger)
	s.Router.Use(middlewares.CreateLogger(s.Logger), middlewares.CreateCompress(s.Logger))

	s.Router.Get("/", api.CreateRootHandler(""))
	s.Router.Route("/update", func(r chi.Router) {
		r.Post("/", api.UpdateRootHandler)
		r.Route("/{type}/{name}/{value}", func(r chi.Router) {
			r.Post("/", api.UpdateHandler)
		})
	})

	s.Router.Route("/updates", func(r chi.Router) {
		r.Post("/", api.UpdateBatchHandler)
	})

	s.Router.Route("/value", func(r chi.Router) {
		r.Post("/", api.ValueRootHandler)
		r.Route("/{type}/{name}", func(r chi.Router) {
			r.Get("/", api.ValueHandler)
		})
	})

	s.Router.Route("/ping", func(r chi.Router) {
		r.Get("/", api.PingHandler)
	})

	s.Handler = s.Router
}
