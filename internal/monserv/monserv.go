package monserv

import (
	"net/http"

	"github.com/Allegathor/perfmon/internal/monserv/handlers"
	"github.com/Allegathor/perfmon/internal/monserv/middlewares"
	"github.com/Allegathor/perfmon/internal/repo/transaction"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type MonServ struct {
	http.Server
	Router        *chi.Mux
	Logger        *zap.SugaredLogger
	txGaugeRepo   transaction.GaugeRepo
	txCounterRepo transaction.CounterRepo
}

func NewInstance(addr string, l *zap.SugaredLogger, gr transaction.GaugeRepo, cr transaction.CounterRepo) *MonServ {
	s := &MonServ{
		Router:        chi.NewRouter(),
		Logger:        l,
		txGaugeRepo:   gr,
		txCounterRepo: cr,
	}

	s.Server = http.Server{Addr: addr}

	return s
}

func (s *MonServ) MountHandlers() {
	r := s.Router
	r.Use(middlewares.CreateLogger(s.Logger), middlewares.Compress)

	r.Get("/", handlers.CreateRootHandler(s.txGaugeRepo, s.txCounterRepo, ""))
	r.Route("/update", func(r chi.Router) {
		r.Post("/", handlers.CreateUpdateRootHandler(s.txGaugeRepo, s.txCounterRepo))
		r.Route("/{type}/{name}/{value}", func(r chi.Router) {
			r.Post("/", handlers.CreateUpdateHandler(s.txGaugeRepo, s.txCounterRepo))
		})
	})
	r.Route("/value", func(r chi.Router) {
		r.Post("/", handlers.CreateValueRootHandler(s.txGaugeRepo, s.txCounterRepo))
		r.Route("/{type}/{name}", func(r chi.Router) {
			r.Get("/", handlers.CreateValueHandler(s.txGaugeRepo, s.txCounterRepo))
		})
	})

	s.Handler = s.Router
}
