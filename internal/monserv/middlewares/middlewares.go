package middlewares

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

type (
	respData struct {
		code int
		size int
	}

	respWriter struct {
		http.ResponseWriter
		respData *respData
	}
)

func (r *respWriter) Write(b []byte) (int, error) {
	size, err := r.ResponseWriter.Write(b)
	r.respData.size += size
	return size, err
}

func (r *respWriter) WriteHeader(code int) {
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
			rwl := &respWriter{
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

type gzipReader struct {
	r  io.ReadCloser
	gr *gzip.Reader
}

func NewGzipReader(r io.ReadCloser) (*gzipReader, error) {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}

	return &gzipReader{
		r:  r,
		gr: gr,
	}, nil
}

func (g *gzipReader) Read(p []byte) (int, error) {
	return g.gr.Read(p)
}

func (g *gzipReader) Close() error {
	if err := g.r.Close(); err != nil {
		return err
	}

	return g.gr.Close()
}

type gzipWriter struct {
	http.ResponseWriter
	writer       *gzip.Writer
	compressible *bool
}

func NewGzipWriter(rw http.ResponseWriter) *gzipWriter {
	return &gzipWriter{
		ResponseWriter: rw,
		writer:         nil,
		compressible:   nil,
	}
}

func (gw *gzipWriter) Write(p []byte) (int, error) {
	if gw.compressible == nil {
		var err error
		ct := gw.Header().Get("Content-Type")
		allowed := strings.HasPrefix(ct, "application/json") || strings.HasPrefix(ct, "text/html")
		gw.compressible = &allowed
		if *gw.compressible {
			gw.Header().Set("Content-Encoding", "gzip")
			gw.writer, err = gzip.NewWriterLevel(gw.ResponseWriter, gzip.BestSpeed)
			if err != nil {
				return 0, err
			}
			return gw.writer.Write(p)
		}

		return gw.ResponseWriter.Write(p)

	} else if *gw.compressible {
		return gw.writer.Write(p)
	}

	return gw.ResponseWriter.Write(p)
}

func (gw *gzipWriter) Close() error {
	if gw.writer != nil {
		return gw.writer.Close()
	}

	return nil
}

func Compress(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if strings.Contains(req.Header.Get("Content-Encoding"), "gzip") {
			gr, err := NewGzipReader(req.Body)
			if err != nil {
				http.Error(rw, "decompression failed", http.StatusInternalServerError)
				return
			}
			req.Body = gr
			defer gr.Close()
		}

		if !strings.Contains(req.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(rw, req)
			return
		}

		gw := NewGzipWriter(rw)
		defer gw.Close()

		next.ServeHTTP(gw, req)
	})
}