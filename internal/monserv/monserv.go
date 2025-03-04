package monserv

import (
	"fmt"
	"net/http"

	"github.com/Allegathor/perfmon/internal/monserv/handlers"
	"github.com/Allegathor/perfmon/internal/storage"
	"github.com/go-chi/chi/v5"
)

type MonServ struct {
	r    chi.Router
	port int
	mux  *http.ServeMux
}

func NewInstance(port int) *MonServ {
	mon := &MonServ{
		r:    chi.NewRouter(),
		port: port,
		mux:  http.NewServeMux(),
	}

	return mon
}

func (mon *MonServ) Run() error {
	addr := fmt.Sprintf(":%d", mon.port)
	ms := storage.NewMetrics()
	mon.r.Get("/", handlers.CreateRootHandler(ms))
	mon.r.Route("/update", func(r chi.Router) {
		r.Route("/{type}/{name}/{value}", func(r chi.Router) {
			r.Post("/", handlers.CreateUpdateHandler(ms))
		})
	})
	mon.r.Route("/value", func(r chi.Router) {
		r.Route("/{type}/{name}", func(r chi.Router) {
			r.Get("/", handlers.CreateValueHandler(ms))
		})
	})

	return http.ListenAndServe(addr, mon.r)
}
