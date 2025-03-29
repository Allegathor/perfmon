package monserv

import (
	"github.com/Allegathor/perfmon/internal/monserv/handlers"
	"github.com/Allegathor/perfmon/internal/monserv/middlewares"
	"github.com/Allegathor/perfmon/internal/repo/transaction"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type MonServ struct {
	Router        *chi.Mux
	Logger        *zap.SugaredLogger
	txGaugeRepo   transaction.GaugeRepo
	txCounterRepo transaction.CounterRepo
}

func NewInstance(addr string, l *zap.SugaredLogger, gr transaction.GaugeRepo, cr transaction.CounterRepo) *MonServ {
	mon := &MonServ{
		Router:        chi.NewRouter(),
		Logger:        l,
		txGaugeRepo:   gr,
		txCounterRepo: cr,
	}

	return mon
}

func (mon *MonServ) MountHandlers() {
	mon.Router.Use(middlewares.CreateLogger(mon.Logger), middlewares.Compress)

	mon.Router.Get("/", handlers.CreateRootHandler(mon.txGaugeRepo, mon.txCounterRepo, ""))
	mon.Router.Route("/update", func(r chi.Router) {
		r.Post("/", handlers.CreateUpdateRootHandler(mon.txGaugeRepo, mon.txCounterRepo))
		r.Route("/{type}/{name}/{value}", func(r chi.Router) {
			r.Post("/", handlers.CreateUpdateHandler(mon.txGaugeRepo, mon.txCounterRepo))
		})
	})
	mon.Router.Route("/value", func(r chi.Router) {
		r.Post("/", handlers.CreateValueRootHandler(mon.txGaugeRepo, mon.txCounterRepo))
		r.Route("/{type}/{name}", func(r chi.Router) {
			r.Get("/", handlers.CreateValueHandler(mon.txGaugeRepo, mon.txCounterRepo))
		})
	})
}
