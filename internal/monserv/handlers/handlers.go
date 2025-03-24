package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
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

type RespError struct {
	err error
	msg string
}

func NewRespError(msg string, err error) *RespError {
	if msg == "" {
		msg = err.Error()
	}

	return &RespError{
		err,
		msg,
	}
}

func (re *RespError) Error() string {
	return re.err.Error()
}

func (re *RespError) Msg() string {
	return re.msg
}

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

func updateMetrics(m *mondata.Metrics, s MetricsStorage) (int, *RespError) {
	var (
		err error
		v   float64
		d   int64
	)

	if m.ID == "" {
		return http.StatusNotFound, NewRespError("name must contain a value", err)
	}

	if m.MType == mondata.GaugeType {
		if m.PValue != "" {
			v, err = mondata.ParseGauge(m.PValue)
			m.Value = &v
			if err != nil {
				return http.StatusBadRequest, NewRespError("invalid value", err)
			}
		}

		s.SetGauge(m.ID, *m.Value)
		return http.StatusOK, nil

	} else if m.MType == mondata.CounterType {
		if m.PValue != "" {
			d, err = mondata.ParseCounter(m.PValue)
			m.Delta = &d
			if err != nil {
				return http.StatusBadRequest, NewRespError("invalid value", err)
			}
		}

		s.SetCounter(m.ID, *m.Delta)
		return http.StatusOK, nil

	}

	return http.StatusBadRequest, NewRespError("", errors.New("incorrect request type"))
}

func CreateUpdateHandler(s MetricsStorage) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		var m = &mondata.Metrics{}

		m.MType = chi.URLParam(req, URLPathType)
		m.ID = chi.URLParam(req, URLPathName)
		m.PValue = chi.URLParam(req, URLPathValue)
		code, err := updateMetrics(m, s)
		if err != nil {
			http.Error(rw, err.Msg(), code)
		}

		rw.WriteHeader(code)
	}
}

func CreateUpdateRootHandler(s MetricsStorage) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		var err error
		m := &mondata.Metrics{}

		if req.Header.Get("Content-Type") == "application/json" {
			var buf bytes.Buffer
			_, err = buf.ReadFrom(req.Body)
			if err != nil {
				http.Error(rw, "reading body failed", http.StatusBadRequest)
				return
			}
			if err = json.Unmarshal(buf.Bytes(), m); err != nil {
				http.Error(rw, "unmarshaling failed", http.StatusBadRequest)
				return
			}

			code, err := updateMetrics(m, s)
			if err != nil {
				http.Error(rw, err.Msg(), code)
			}

			rw.WriteHeader(code)
		} else {
			http.Error(rw, "unsupported content type", http.StatusBadRequest)
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
