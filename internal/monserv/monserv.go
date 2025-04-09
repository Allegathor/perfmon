package monserv

import (
	"net/http"

	"github.com/Allegathor/perfmon/internal/monserv/handlers"
	"github.com/Allegathor/perfmon/internal/monserv/middlewares"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type MonServ struct {
	http.Server
	db     handlers.MDB
	Router *chi.Mux
	Logger *zap.SugaredLogger
}

func NewInstance(addr string, db handlers.MDB, l *zap.SugaredLogger) *MonServ {
	s := &MonServ{
		db:     db,
		Router: chi.NewRouter(),
		Logger: l,
	}

	s.Server = http.Server{Addr: addr}

	return s
}

func (s *MonServ) MountHandlers() {
	r := s.Router
	r.Use(middlewares.CreateLogger(s.Logger), middlewares.Compress)

	r.Get("/", handlers.CreateRootHandler(s.db, ""))
	r.Route("/update", func(r chi.Router) {
		r.Post("/", handlers.CreateUpdateRootHandler(s.db))
		r.Route("/{type}/{name}/{value}", func(r chi.Router) {
			r.Post("/", handlers.CreateUpdateHandler(s.db))
		})
	})
	r.Route("/value", func(r chi.Router) {
		r.Post("/", handlers.CreateValueRootHandler(s.db))
		r.Route("/{type}/{name}", func(r chi.Router) {
			r.Get("/", handlers.CreateValueHandler(s.db))
		})
	})
	r.Route("/ping", func(r chi.Router) {
		r.Get("/", handlers.CreatePingHandler(s.db))
	})

	s.Handler = s.Router
}
