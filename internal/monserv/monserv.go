package monserv

import (
	"github.com/Allegathor/perfmon/internal/monserv/handlers"
	"github.com/Allegathor/perfmon/internal/monserv/middlewares"
	"github.com/Allegathor/perfmon/internal/storage"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type MonServ struct {
	Router *chi.Mux
	Logger *zap.SugaredLogger
}

func NewInstance(addr string, l *zap.SugaredLogger) *MonServ {
	mon := &MonServ{
		Router: chi.NewRouter(),
		Logger: l,
	}

	return mon
}

func (mon *MonServ) MountHandlers() {
	mon.Router.Use(middlewares.CreateLogger(mon.Logger))

	ms := storage.NewMetrics()
	mon.Router.Get("/", handlers.CreateRootHandler(ms, ""))
	mon.Router.Route("/update", func(r chi.Router) {
		r.Route("/{type}/{name}/{value}", func(r chi.Router) {
			r.Post("/", handlers.CreateUpdateHandler(ms))
		})
	})
	mon.Router.Route("/value", func(r chi.Router) {
		r.Route("/{type}/{name}", func(r chi.Router) {
			r.Get("/", handlers.CreateValueHandler(ms))
		})
	})
}
