package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strings"

	"github.com/Allegathor/perfmon/internal/mondata"
	"github.com/Allegathor/perfmon/internal/repo/transaction"
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

func CreateRootHandler(gr transaction.GaugeRepo, cr transaction.CounterRepo, path string) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		type Vals interface {
			map[string]float64 | map[string]int64
		}

		type Table[T Vals] struct {
			Name    string
			Content T
		}

		gch := make(chan map[string]float64)
		cch := make(chan map[string]int64)

		go gr.Read(func(tx transaction.Tx[float64]) error {
			gch <- tx.GetAll()
			return nil
		})

		go cr.Read(func(tx transaction.Tx[int64]) error {
			cch <- tx.GetAll()
			tx.GetAll()

			return nil
		})

		gVals := <-gch
		cVals := <-cch

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

func updateMetrics(m *mondata.Metrics, gr transaction.GaugeRepo, cr transaction.CounterRepo) (int, *RespError) {
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

		go gr.Update(func(tx transaction.Tx[float64]) error {
			tx.Set(m.ID, *m.Value)
			return nil
		})

		return http.StatusOK, nil

	} else if m.MType == mondata.CounterType {
		if m.SValue != "" {
			d, err := mondata.ParseCounter(m.SValue)
			if err != nil {
				return http.StatusBadRequest, NewRespError("invalid value", err)
			}
			m.Delta = &d
		}

		go cr.Update(func(tx transaction.Tx[int64]) error {
			tx.SetAccum(m.ID, *m.Delta)
			return nil
		})
		return http.StatusOK, nil

	}

	return http.StatusBadRequest, NewRespError("incorrect request type", nil)
}

func CreateUpdateHandler(gr transaction.GaugeRepo, cr transaction.CounterRepo) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		var m = &mondata.Metrics{}

		m.MType = chi.URLParam(req, URLPathType)
		m.ID = chi.URLParam(req, URLPathName)
		m.SValue = chi.URLParam(req, URLPathValue)
		code, err := updateMetrics(m, gr, cr)
		if err != nil {
			http.Error(rw, err.Msg(), code)
			return
		}

		rw.WriteHeader(code)
	}
}

func CreateUpdateRootHandler(gr transaction.GaugeRepo, cr transaction.CounterRepo) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		if strings.Contains(req.Header.Get("Content-Type"), "application/json") {
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

			code, respErr := updateMetrics(m, gr, cr)
			if respErr != nil {
				http.Error(rw, respErr.Msg(), code)
				return
			}

			rw.WriteHeader(code)
		} else {
			http.Error(rw, "unsupported content type", http.StatusBadRequest)
		}
		fmt.Printf("value[%s] with type %s doesn't exist in the storage\n", m.MType, m.ID)
		return &vhData{code: http.StatusNotFound}, NewRespError("value doesn't exist in the storage", nil)
	} else if m.MType == mondata.CounterType {
		var (
			v      int64
			ok     bool
			ch     = make(chan int64)
			chbool = make(chan bool)
		)

		go cr.Read(func(tx repo.Tx[int64]) error {
			value, found := tx.Get(m.ID)
			ch <- value
			chbool <- found
			return nil
		})

		v, ok = <-ch, <-chbool
		if ok {
			return &vhData{
				code: http.StatusOK,
				metrics: &mondata.Metrics{
					ID: m.ID, MType: m.MType, Delta: &v, SValue: mondata.FormatCounter(v),
				},
			}, nil
		}
		fmt.Printf("value[%s] with type %s doesn't exist in the storage\n", m.MType, m.ID)
		return &vhData{code: http.StatusNotFound}, NewRespError("value doesn't exist in the storage", nil)
	}

	return &vhData{code: http.StatusBadRequest}, NewRespError("incorrect request type", nil)
}

type vhData struct {
	metrics *mondata.Metrics
	code    int
}

func getVhData(m *mondata.Metrics, gr transaction.GaugeRepo, cr transaction.CounterRepo) (*vhData, *RespError) {
	if m.ID == "" {
		return &vhData{code: http.StatusNotFound}, NewRespError("name must contain a value", nil)
	}

	if m.MType == mondata.GaugeType {
		var (
			v      float64
			ok     bool
			ch     = make(chan float64)
			chbool = make(chan bool)
		)

		go gr.Read(func(tx transaction.Tx[float64]) error {
			value, found := tx.Get(m.ID)
			ch <- value
			chbool <- found
			return nil
		})

		v, ok = <-ch, <-chbool
		if ok {
			return &vhData{
				code: http.StatusOK,
				metrics: &mondata.Metrics{
					ID: m.ID, MType: m.MType, Value: &v, SValue: mondata.FormatGauge(v),
				},
			}, nil
		}
		fmt.Printf("value[%s] with type %s doesn't exist in the storage\n", m.MType, m.ID)
		return &vhData{code: http.StatusNotFound}, NewRespError("value doesn't exist in the storage", nil)
	} else if m.MType == mondata.CounterType {
		var (
			v      int64
			ok     bool
			ch     = make(chan int64)
			chbool = make(chan bool)
		)

		go cr.Read(func(tx transaction.Tx[int64]) error {
			value, found := tx.Get(m.ID)
			ch <- value
			chbool <- found
			return nil
		})

		v, ok = <-ch, <-chbool
		if ok {
			return &vhData{
				code: http.StatusOK,
				metrics: &mondata.Metrics{
					ID: m.ID, MType: m.MType, Delta: &v, SValue: mondata.FormatCounter(v),
				},
			}, nil
		}
		fmt.Printf("value[%s] with type %s doesn't exist in the storage\n", m.MType, m.ID)
		return &vhData{code: http.StatusNotFound}, NewRespError("value doesn't exist in the storage", nil)
	}

	return &vhData{code: http.StatusBadRequest}, NewRespError("incorrect request type", nil)
}

func CreateValueHandler(gr transaction.GaugeRepo, cr transaction.CounterRepo) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		m := &mondata.Metrics{}
		m.MType = chi.URLParam(req, URLPathType)
		m.ID = chi.URLParam(req, URLPathName)
		vhd, respErr := getVhData(m, gr, cr)
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

func CreateValueRootHandler(gr transaction.GaugeRepo, cr transaction.CounterRepo) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		if strings.Contains(req.Header.Get("Content-Type"), "application/json") {
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

			vhd, respErr := getVhData(m, gr, cr)
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
		}
	}
}
