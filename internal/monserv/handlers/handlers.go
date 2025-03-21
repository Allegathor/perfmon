package handlers

import (
	"html/template"
	"net/http"
	"os"

	"github.com/Allegathor/perfmon/internal/mondata"
	"github.com/go-chi/chi/v5"
)

const (
	URLPathType  = "type"
	URLPathName  = "name"
	URLPathValue = "value"
)

type MetricsStorage interface {
	SetGauge(string, float64)
	GetGauge(string) (float64, bool)
	GetGaugeAll() map[string]float64

	SetCounter(string, int64)
	GetCounter(string) (int64, bool)
	GetCounterAll() map[string]int64
}

func CreateRootHandler(s MetricsStorage, path string) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		type Value interface {
			map[string]float64 | map[string]int64
		}
		type Table[T Value] struct {
			Name    string
			Content T
		}

		viewData := []any{
			Table[map[string]int64]{Name: "Counter", Content: s.GetCounterAll()},
			Table[map[string]float64]{Name: "Gauge", Content: s.GetGaugeAll()},
		}

		if path == "" {
			dir, _ := os.Getwd()
			path = dir + "/templates/index.html"
		}

		t, err := template.New("index.html").ParseFiles(path)
		if err != nil {
			http.Error(rw, "file parsing error", http.StatusInternalServerError)
			return
		}

		err = t.Execute(rw, viewData)
		if err != nil {
			http.Error(rw, "template execution error", http.StatusInternalServerError)
		}
	}
}

func CreateUpdateHandler(s MetricsStorage) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {

		t := chi.URLParam(req, URLPathType)
		n := chi.URLParam(req, URLPathName)
		v := chi.URLParam(req, URLPathValue)

		if n == "" {
			http.Error(rw, "name must contain a value", http.StatusNotFound)
			return
		}

		if t == mondata.GaugeType {
			gv, err := mondata.ParseGauge(v)
			if err != nil {
				http.Error(rw, "invalid value", http.StatusBadRequest)
				return
			}
			s.SetGauge(n, gv)
			rw.WriteHeader(http.StatusOK)

		} else if t == mondata.CounterType {
			cv, err := mondata.ParseCounter(v)
			if err != nil {
				http.Error(rw, "invalid value", http.StatusBadRequest)
				return
			}
			s.SetCounter(n, cv)
			rw.WriteHeader(http.StatusOK)

		} else {
			http.Error(rw, "incorrect request type", http.StatusBadRequest)
		}

	}
}

func CreateValueHandler(s MetricsStorage) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		t := chi.URLParam(req, URLPathType)
		n := chi.URLParam(req, URLPathName)

		if n == "" {
			http.Error(rw, "name must contain a value", http.StatusNotFound)
			return
		}

		if t == mondata.GaugeType {

			if v, ok := s.GetGauge(n); ok {
				_, err := rw.Write([]byte(mondata.FormatGauge(v)))
				if err != nil {
					http.Error(rw, "rw error", http.StatusInternalServerError)
				}
				return
			}

			http.Error(rw, "value doesn't exist in storage", http.StatusNotFound)

		} else if t == mondata.CounterType {
			if v, ok := s.GetCounter(n); ok {
				_, err := rw.Write([]byte(mondata.FormatCounter(v)))
				if err != nil {
					http.Error(rw, "rw error", http.StatusInternalServerError)
				}
				return
			}

			http.Error(rw, "value doesn't exist in storage", http.StatusNotFound)

		} else {
			http.Error(rw, "incorrect request type", http.StatusBadRequest)
		}

	}
}
