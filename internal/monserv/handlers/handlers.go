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
	var e = err
	if err == nil {
		e = errors.New(msg)
	}

	return &RespError{
		e,
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
	if m.ID == "" {
		return http.StatusNotFound, NewRespError("name must contain a value", nil)
	}

	if m.MType == mondata.GaugeType {
		if m.SValue != "" {
			v, err := mondata.ParseGauge(m.SValue)
			if err != nil {
				return http.StatusBadRequest, NewRespError("invalid value", err)
			}
			m.Value = &v
		}

		s.SetGauge(m.ID, *m.Value)
		return http.StatusOK, nil

	} else if m.MType == mondata.CounterType {
		if m.SValue != "" {
			d, err := mondata.ParseCounter(m.SValue)
			if err != nil {
				return http.StatusBadRequest, NewRespError("invalid value", err)
			}
			m.Delta = &d
		}

		s.SetCounter(m.ID, *m.Delta)
		return http.StatusOK, nil

	}

	return http.StatusBadRequest, NewRespError("incorrect request type", nil)
}

func CreateUpdateHandler(s MetricsStorage) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		var m = &mondata.Metrics{}

		m.MType = chi.URLParam(req, URLPathType)
		m.ID = chi.URLParam(req, URLPathName)
		m.SValue = chi.URLParam(req, URLPathValue)
		code, err := updateMetrics(m, s)
		if err != nil {
			http.Error(rw, err.Msg(), code)
			return
		}

		rw.WriteHeader(code)
	}
}

func CreateUpdateRootHandler(s MetricsStorage) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Content-Type") == "application/json" {
			var buf bytes.Buffer

			_, err := buf.ReadFrom(req.Body)
			if err != nil {
				http.Error(rw, "reading body failed", http.StatusBadRequest)
				return
			}

			m := &mondata.Metrics{}
			if err := json.Unmarshal(buf.Bytes(), m); err != nil {
				http.Error(rw, "unmarshaling failed", http.StatusBadRequest)
				return
			}

			code, respErr := updateMetrics(m, s)
			if respErr != nil {
				http.Error(rw, respErr.Msg(), code)
				return
			}

			rw.WriteHeader(code)
		} else {
			http.Error(rw, "unsupported content type", http.StatusBadRequest)
		}

	}
}

type vhData struct {
	metrics *mondata.Metrics
	code    int
}

func getVhData(m *mondata.Metrics, s MetricsStorage) (*vhData, *RespError) {
	if m.ID == "" {
		return &vhData{code: http.StatusNotFound}, NewRespError("name must contain a value", nil)
	}

	if m.MType == mondata.GaugeType {
		if v, ok := s.GetGauge(m.ID); ok {
			return &vhData{
				code: http.StatusOK,
				metrics: &mondata.Metrics{
					ID: m.ID, MType: m.MType, Value: &v, SValue: mondata.FormatGauge(v),
				},
			}, nil
		}
		return &vhData{code: http.StatusNotFound}, NewRespError("value doesn't exist in storage", nil)
	} else if m.MType == mondata.CounterType {
		if v, ok := s.GetCounter(m.ID); ok {
			return &vhData{
				code: http.StatusOK,
				metrics: &mondata.Metrics{
					ID: m.ID, MType: m.MType, Delta: &v, SValue: mondata.FormatCounter(v),
				},
			}, nil
		}
		return &vhData{code: http.StatusNotFound}, NewRespError("value doesn't exist in storage", nil)
	}

	return &vhData{code: http.StatusBadRequest}, NewRespError("incorrect request type", nil)
}

func CreateValueHandler(s MetricsStorage) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		m := &mondata.Metrics{}
		m.MType = chi.URLParam(req, URLPathType)
		m.ID = chi.URLParam(req, URLPathName)
		vhd, respErr := getVhData(m, s)
		if respErr != nil {
			http.Error(rw, respErr.Msg(), vhd.code)
			return
		}

		_, err := rw.Write([]byte(vhd.metrics.SValue))
		if err != nil {
			http.Error(rw, "rw error", http.StatusInternalServerError)
			return
		}
	}
}

func CreateValueRootHandler(s MetricsStorage) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Content-Type") == "application/json" {
			var buf bytes.Buffer
			_, err := buf.ReadFrom(req.Body)
			if err != nil {
				http.Error(rw, "reading body failed", http.StatusBadRequest)
				return
			}

			m := &mondata.Metrics{}
			if err := json.Unmarshal(buf.Bytes(), m); err != nil {
				http.Error(rw, "unmarshaling failed", http.StatusBadRequest)
				return
			}

			vhd, respErr := getVhData(m, s)
			if respErr != nil {
				http.Error(rw, respErr.Msg(), vhd.code)
				return
			}

			b, err := json.Marshal(vhd.metrics)
			if err != nil {
				http.Error(rw, "marshaling failed", http.StatusInternalServerError)
				return
			}

			rw.Header().Add("Content-Type", "application/json")
			_, err = rw.Write(b)
			if err != nil {
				http.Error(rw, "rw error", http.StatusInternalServerError)
				return
			}
		} else {
			http.Error(rw, "unsupported content type", http.StatusBadRequest)
		}
	}
}
