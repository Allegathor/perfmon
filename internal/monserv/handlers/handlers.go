package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	defcfg "github.com/Allegathor/perfmon/internal"
	"github.com/Allegathor/perfmon/internal/storage"
)

type Number interface {
	int64 | float64
}

func CreateUpdateHandler(s *storage.MetricsStorage) http.HandlerFunc {

	return func(rw http.ResponseWriter, req *http.Request) {
		t := req.PathValue(defcfg.URLTypePath)
		n := req.PathValue(defcfg.URLNamePath)
		v := req.PathValue(defcfg.URLValuePath)
		if t == defcfg.UpdateTypeGauge {
			gv, err := strconv.ParseFloat(v, 64)
			if err != nil {
				http.Error(rw, "internal error", http.StatusInternalServerError)
			}
			s.Add(storage.MetricRec{ValueType: t, Name: n, GaugeVal: gv})
			rw.WriteHeader(http.StatusOK)

		} else if t == defcfg.UpdateTypeCounter {
			cv, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				http.Error(rw, "internal error", http.StatusInternalServerError)
			}
			s.Add(storage.MetricRec{ValueType: t, Name: n, CounterVal: cv})
			rw.WriteHeader(http.StatusOK)

		} else {
			http.Error(rw, "incorrect request type", http.StatusBadRequest)
		}

	}
}

func CreateHistoryHandler(s *storage.MetricsStorage) http.HandlerFunc {

	return func(rw http.ResponseWriter, req *http.Request) {
		var body string
		for _, v := range s.GetHistory() {
			body += fmt.Sprintf("%v", v)
		}
		rw.Write([]byte(body))
	}
}
