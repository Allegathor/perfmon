package monserv

import (
	"github.com/Allegathor/perfmon/internal/monserv/handlers"
	"github.com/Allegathor/perfmon/internal/monserv/middlewares"
	"github.com/Allegathor/perfmon/internal/repo"
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

	gr := repo.NewMRepo[float64]()
	cr := repo.NewMRepo[int64]()

	mon.Router.Get("/", handlers.CreateRootHandler(gr, cr, ""))
	mon.Router.Route("/update", func(r chi.Router) {
		r.Post("/", handlers.CreateUpdateRootHandler(gr, cr))
		r.Route("/{type}/{name}/{value}", func(r chi.Router) {
			r.Post("/", handlers.CreateUpdateHandler(gr, cr))
		})
	})
	mon.Router.Route("/value", func(r chi.Router) {
		r.Post("/", handlers.CreateValueRootHandler(gr, cr))
		r.Route("/{type}/{name}", func(r chi.Router) {
			r.Get("/", handlers.CreateValueHandler(gr, cr))
		})
	})
}
