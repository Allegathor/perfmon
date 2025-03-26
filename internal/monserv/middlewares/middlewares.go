package middlewares

import (
	"net/http"
	"time"

	"go.uber.org/zap"
)

type (
	respData struct {
		code int
		size int
	}

	loggerRespWriter struct {
		http.ResponseWriter
		respData *respData
	}
)

func (r *loggerRespWriter) Write(b []byte) (int, error) {
	size, err := r.ResponseWriter.Write(b)
	r.respData.size += size
	return size, err
}

func (r *loggerRespWriter) WriteHeader(code int) {
	r.ResponseWriter.WriteHeader(code)
	r.respData.code = code
}

func CreateLogger(s *zap.SugaredLogger) func(http.Handler) http.Handler {

	return func(next http.Handler) http.Handler {
		logFn := func(rw http.ResponseWriter, req *http.Request) {
			start := time.Now()
			u := req.RequestURI
			m := req.Method
			h := req.Header
			r := &respData{code: 0, size: 0}
			rwl := &loggerRespWriter{
				ResponseWriter: rw,
				respData:       r,
			}

			next.ServeHTTP(rwl, req)
			d := time.Since(start)
			s.Infoln(
				"uri:", u,
				"method:", m,
				"headers:", h,
				"duration:", d,
				"resp code/size:", r.code, r.size,
			)
		}

		return http.HandlerFunc(logFn)
	}

}
