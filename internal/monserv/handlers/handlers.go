package handlers

import (
	"html/template"
	"net/http"
	"os"
	"strconv"

	defcfg "github.com/Allegathor/perfmon/internal"
	"github.com/Allegathor/perfmon/internal/storage"
	"github.com/go-chi/chi/v5"
)

func CreateRootHandler(s *storage.MetricsStorage, path string) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		type Value interface {
			storage.Gauge | storage.Counter
		}
		type Table[T Value] struct {
			Name    string
			Content T
		}

		viewData := []any{
			Table[storage.Counter]{Name: "Counter", Content: s.Counter},
			Table[storage.Gauge]{Name: "Gauge", Content: s.Gauge},
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

func CreateUpdateHandler(s *storage.MetricsStorage) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {

		t := chi.URLParam(req, defcfg.URLTypePath)
		n := chi.URLParam(req, defcfg.URLNamePath)
		v := chi.URLParam(req, defcfg.URLValuePath)

		if n == "" {
			http.Error(rw, "name must contain a value", http.StatusNotFound)
			return
		}

		if t == defcfg.TypeGauge {
			gv, err := strconv.ParseFloat(v, 64)
			if err != nil {
				http.Error(rw, "invalid value", http.StatusBadRequest)
				return
			}
			s.SetGauge(n, gv)
			rw.WriteHeader(http.StatusOK)

		} else if t == defcfg.TypeCounter {
			cv, err := strconv.ParseInt(v, 10, 64)
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

func CreateValueHandler(s *storage.MetricsStorage) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		t := req.PathValue(defcfg.URLTypePath)
		n := req.PathValue(defcfg.URLNamePath)

		if n == "" {
			http.Error(rw, "name must contain a value", http.StatusNotFound)
			return
		}

		if t == defcfg.TypeGauge {

			if v, ok := s.Gauge[n]; ok {
				_, err := rw.Write([]byte(strconv.FormatFloat(v, 'f', -1, 64)))
				if err != nil {
					http.Error(rw, "rw error", http.StatusInternalServerError)
				}
				return
			}

			http.Error(rw, "value doesn't exist in storage", http.StatusNotFound)

		} else if t == defcfg.TypeCounter {
			if v, ok := s.Counter[n]; ok {
				_, err := rw.Write([]byte(strconv.FormatInt(v, 10)))
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
