package monserv

import (
	"fmt"
	"net/http"

	"github.com/Allegathor/perfmon/internal/monserv/handlers"
	"github.com/Allegathor/perfmon/internal/storage"
)

type MonServ struct {
	port int
	mux  *http.ServeMux
}

func NewInstance(port int) *MonServ {
	s := &MonServ{
		port: port,
		mux:  http.NewServeMux(),
	}

	return s
}

func (s *MonServ) Run() error {
	addr := fmt.Sprintf(":%d", s.port)
	ms := storage.NewMetrics()
	s.mux.HandleFunc("/update/{type}/{name}/{value}", handlers.CreateUpdateHandler(ms))
	s.mux.HandleFunc("/history/", handlers.CreateHistoryHandler(ms))

	return http.ListenAndServe(addr, s.mux)
}
