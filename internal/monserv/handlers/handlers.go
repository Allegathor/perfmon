package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	defcfg "github.com/Allegathor/perfmon/internal"
	"github.com/Allegathor/perfmon/internal/storage"
)

func CreateUpdateHandler(s *storage.MetricsStorage) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			rw.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		t := req.PathValue(defcfg.URLTypePath)
		n := req.PathValue(defcfg.URLNamePath)
		v := req.PathValue(defcfg.URLValuePath)

		if n == "" {
			http.Error(rw, "name must contain a value", http.StatusNotFound)
		}

		if t == defcfg.UpdateTypeGauge {
			gv, err := strconv.ParseFloat(v, 64)
			if err != nil {
				http.Error(rw, "invalid value", http.StatusBadRequest)
			}
			s.Add(storage.MetricRec{ValueType: t, Name: n, GaugeVal: gv})
			rw.WriteHeader(http.StatusOK)

		} else if t == defcfg.UpdateTypeCounter {
			cv, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				http.Error(rw, "invalid value", http.StatusBadRequest)
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
