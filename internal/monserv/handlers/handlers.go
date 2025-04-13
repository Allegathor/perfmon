package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"os"
	"strings"

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

type Getters interface {
	GetGauge(ctx context.Context, name string) (mondata.GaugeVType, bool, error)
	GetGaugeAll(ctx context.Context) (mondata.GaugeMap, error)

	GetCounter(ctx context.Context, name string) (mondata.CounterVType, bool, error)
	GetCounterAll(ctx context.Context) (mondata.CounterMap, error)
}

type Setters interface {
	SetGauge(ctx context.Context, name string, value mondata.GaugeVType) error
	SetGaugeAll(ctx context.Context, gaugeMap mondata.GaugeMap) error

	SetCounter(ctx context.Context, name string, value mondata.CounterVType) error
	SetCounterAll(ctx context.Context, gaugeMap mondata.CounterMap) error
}

type MDB interface {
	Getters
	Setters
	Ping(ctx context.Context) error
}

func CreateRootHandler(db MDB, path string) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		type Vals interface {
			map[string]float64 | map[string]int64
		}

		type Table[T Vals] struct {
			Name    string
			Content T
		}

		gVals, err := db.GetGaugeAll(req.Context())
		if err != nil {
			http.Error(rw, "an error occured while acquaring gauge values from db", http.StatusInternalServerError)
			return
		}

		cVals, err := db.GetCounterAll(req.Context())
		if err != nil {
			http.Error(rw, "an error occured while acquaring counter values from db", http.StatusInternalServerError)
			return
		}

		viewData := []any{
			Table[map[string]float64]{Name: "Gauge", Content: gVals},
			Table[map[string]int64]{Name: "Counter", Content: cVals},
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

		rw.Header().Set("Content-Type", "text/html; charset=utf-8")
		err = t.Execute(rw, viewData)
		if err != nil {
			http.Error(rw, "template execution error", http.StatusInternalServerError)
		}
	}
}

func updateMetrics(ctx context.Context, m *mondata.Metrics, db MDB) (int, *RespError) {
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

		err := db.SetGauge(ctx, m.ID, *m.Value)
		if err != nil {
			return http.StatusInternalServerError, NewRespError("setting gauge value in db failed", nil)
		}

		return http.StatusOK, nil

	} else if m.MType == mondata.CounterType {
		if m.SValue != "" {
			d, err := mondata.ParseCounter(m.SValue)
			if err != nil {
				return http.StatusBadRequest, NewRespError("invalid value", err)
			}
			m.Delta = &d
		}

		err := db.SetCounter(ctx, m.ID, *m.Delta)
		if err != nil {
			return http.StatusInternalServerError, NewRespError("setting counter value in db failed", nil)
		}
		return http.StatusOK, nil

	}

	return http.StatusBadRequest, NewRespError("incorrect request type", nil)
}

func CreateUpdateHandler(db MDB) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		var m = &mondata.Metrics{}

		m.MType = chi.URLParam(req, URLPathType)
		m.ID = chi.URLParam(req, URLPathName)
		m.SValue = chi.URLParam(req, URLPathValue)
		code, err := updateMetrics(req.Context(), m, db)
		if err != nil {
			http.Error(rw, err.Msg(), code)
			return
		}

		rw.WriteHeader(code)
	}
}

func CreateUpdateRootHandler(db MDB) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		if strings.Contains(req.Header.Get("Content-Type"), "application/json") {
			var buf bytes.Buffer

			_, err := buf.ReadFrom(req.Body)
			if err != nil {
				http.Error(rw, "working with request body failed", http.StatusBadRequest)
				return
			}

			defer func() {
				err := req.Body.Close()
				if err != nil {
					http.Error(rw, "working with request body failed", http.StatusInternalServerError)
					return
				}
			}()

			m := &mondata.Metrics{}
			if err := json.Unmarshal(buf.Bytes(), m); err != nil {
				http.Error(rw, "unmarshaling failed", http.StatusBadRequest)
				return
			}

			code, respErr := updateMetrics(req.Context(), m, db)
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

func CreateUpdateBatchHandler(db MDB) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		if strings.Contains(req.Header.Get("Content-Type"), "application/json") {
			var buf bytes.Buffer

			_, err := buf.ReadFrom(req.Body)
			if err != nil {
				http.Error(rw, "working with request body failed", http.StatusBadRequest)
				return
			}

			defer func() {
				err := req.Body.Close()
				if err != nil {
					http.Error(rw, "working with request body failed", http.StatusInternalServerError)
					return
				}
			}()

			mm := &[]mondata.Metrics{}
			if err := json.Unmarshal(buf.Bytes(), mm); err != nil {
				http.Error(rw, "unmarshaling failed", http.StatusBadRequest)
				return
			}

			gm := make(map[string]float64)
			cm := make(map[string]int64)

			for _, rec := range *mm {
				if rec.ID == "" {
					continue
				}

				if rec.MType == mondata.GaugeType {
					gm[rec.ID] = *rec.Value

				} else if rec.MType == mondata.CounterType {
					cm[rec.ID] = *rec.Delta
				}
			}

			if len(gm) == 0 && len(cm) == 0 {
				http.Error(rw, "nothing to update", http.StatusBadRequest)
				return
			}

			if len(gm) > 0 {
				if err := db.SetGaugeAll(req.Context(), gm); err != nil {
					http.Error(rw, "gauge batch update to db failed", http.StatusInternalServerError)
					return
				}
			}

			if len(cm) > 0 {
				if err := db.SetCounterAll(req.Context(), cm); err != nil {
					http.Error(rw, "counter batch update to db failed", http.StatusInternalServerError)
					return
				}
			}

			rw.WriteHeader(http.StatusOK)

		} else {
			http.Error(rw, "unsupported content type", http.StatusBadRequest)
		}
	}
}

type vhData struct {
	metrics *mondata.Metrics
	code    int
}

func getVhData(ctx context.Context, m *mondata.Metrics, db MDB) (*vhData, *RespError) {
	if m.ID == "" {
		return &vhData{code: http.StatusNotFound}, NewRespError("name must contain a value", nil)
	}

	if m.MType == mondata.GaugeType {
		v, ok, err := db.GetGauge(ctx, m.ID)
		if err != nil {
			return &vhData{code: http.StatusInternalServerError}, NewRespError("getting gauge value from db failed", nil)
		} else if ok {
			return &vhData{
				code: http.StatusOK,
				metrics: &mondata.Metrics{
					ID: m.ID, MType: m.MType, Value: &v, SValue: mondata.FormatGauge(v),
				},
			}, nil
		}
		return &vhData{code: http.StatusNotFound}, NewRespError("value doesn't exist in the storage", nil)
	} else if m.MType == mondata.CounterType {
		v, ok, err := db.GetCounter(ctx, m.ID)
		if err != nil {
			return &vhData{code: http.StatusInternalServerError}, NewRespError("getting counter value from db failed", nil)
		} else if ok {
			return &vhData{
				code: http.StatusOK,
				metrics: &mondata.Metrics{
					ID: m.ID, MType: m.MType, Delta: &v, SValue: mondata.FormatCounter(v),
				},
			}, nil
		}
		return &vhData{code: http.StatusNotFound}, NewRespError("value doesn't exist in the storage", nil)
	}

	return &vhData{code: http.StatusBadRequest}, NewRespError("incorrect request type", nil)
}

func CreateValueHandler(db MDB) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		m := &mondata.Metrics{}
		m.MType = chi.URLParam(req, URLPathType)
		m.ID = chi.URLParam(req, URLPathName)
		vhd, respErr := getVhData(req.Context(), m, db)
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

func CreateValueRootHandler(db MDB) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		if strings.Contains(req.Header.Get("Content-Type"), "application/json") {
			var buf bytes.Buffer
			_, err := buf.ReadFrom(req.Body)
			if err != nil {
				http.Error(rw, "working with request body failed", http.StatusBadRequest)
				return
			}

			defer func() {
				err := req.Body.Close()
				if err != nil {
					http.Error(rw, "working with request body failed", http.StatusInternalServerError)
					return
				}
			}()

			m := &mondata.Metrics{}
			err = json.Unmarshal(buf.Bytes(), m)
			if err != nil {
				http.Error(rw, "unmarshaling failed", http.StatusBadRequest)
				return
			}

			vhd, respErr := getVhData(req.Context(), m, db)
			if respErr != nil {
				http.Error(rw, respErr.Msg(), vhd.code)
				return
			}

			b, err := json.Marshal(vhd.metrics)
			if err != nil {
				http.Error(rw, "marshaling failed", http.StatusInternalServerError)
				return
			}

			rw.Header().Add("Content-Type", "application/json; charset=utf-8")
			_, err = rw.Write(b)
			if err != nil {
				http.Error(rw, "rw error", http.StatusInternalServerError)
				return
			}
		} else {
			http.Error(rw, "unsupported content type", http.StatusBadRequest)
			return
		}
	}
}

func CreatePingHandler(db MDB) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		err := db.Ping(req.Context())
		if err != nil {
			http.Error(rw, "connection to DB wasn't established", http.StatusInternalServerError)
			return
		}

		rw.WriteHeader(http.StatusOK)
	}
}
