package monserv

import (
	"github.com/Allegathor/perfmon/internal/monserv/handlers"
	"github.com/Allegathor/perfmon/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type MonServ struct {
	Router *chi.Mux
}

func NewInstance(addr string) *MonServ {
	mon := &MonServ{
		Router: chi.NewRouter(),
	}

	return mon
}

func (mon *MonServ) MountHandlers() {
	mon.Router.Use(middleware.Logger)

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
