package monserv

import (
	"context"
	"crypto/rsa"
	"net"
	"net/http"

	"github.com/Allegathor/perfmon/internal/monserv/handlers"
	"github.com/Allegathor/perfmon/internal/monserv/middlewares"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

type MonServ struct {
	*http.Server
	db            handlers.MDB
	key           string
	cryptoKey     *rsa.PrivateKey
	trustedSubnet string
	Router        *chi.Mux
	Logger        *zap.SugaredLogger
}

func NewInstance(ctx context.Context, addr string, db handlers.MDB, key string, cryptoKey *rsa.PrivateKey, subnet string, l *zap.SugaredLogger) *MonServ {
	s := &MonServ{
		db:            db,
		key:           key,
		cryptoKey:     cryptoKey,
		trustedSubnet: subnet,
		Router:        chi.NewRouter(),
		Logger:        l,
	}

	s.Server = &http.Server{Addr: addr, BaseContext: func(l net.Listener) context.Context {
		return ctx
	}}

	return s
}

func (s *MonServ) MountHandlers() {
	api := handlers.NewAPI(s.db, s.Logger)
	s.Router.Mount("/debug/", middleware.Profiler())
	mw := chi.Middlewares{middlewares.CreateLogger(s.Logger), middlewares.CreateUncompressReq(s.Logger)}
	if s.trustedSubnet != "" {
		mw = append(mw, middlewares.CreateSubnetRestrictor(s.trustedSubnet, s.Logger))
	}
	// update-related middlewares
	umw := make(chi.Middlewares, len(mw))
	copy(umw, mw)

	if s.key != "" {
		umw = append(umw, middlewares.CreateSumChecker(s.key, s.Logger))
		umw = append(umw, middlewares.CreateSigner(s.key, s.Logger))
	}

	if s.cryptoKey != nil {
		umw = append(umw, middlewares.CreateMsgDecrypter(s.cryptoKey, s.Logger))
	}

	mw = append(mw, middlewares.CreateCompress(s.Logger))
	umw = append(umw, middlewares.CreateCompress(s.Logger))

	// main group
	s.Router.Group(func(r chi.Router) {
		r.Use(mw...)
		r.Get("/", api.CreateRootHandler(""))

		r.Route("/value", func(r chi.Router) {
			r.Post("/", api.ValueRootHandler)
			r.Route("/{type}/{name}", func(r chi.Router) {
				r.Get("/", api.ValueHandler)
			})
		})

		r.Route("/ping", func(r chi.Router) {
			r.Get("/", api.PingHandler)
		})
	})

	// update group
	s.Router.Group(func(r chi.Router) {
		r.Use(umw...)

		r.Route("/update", func(r chi.Router) {
			r.Post("/", api.UpdateRootHandler)
			r.Route("/{type}/{name}/{value}", func(r chi.Router) {
				r.Post("/", api.UpdateHandler)
			})
		})

		r.Route("/updates", func(r chi.Router) {
			r.Post("/", api.UpdateBatchHandler)
		})
	})

	s.Handler = s.Router
}
