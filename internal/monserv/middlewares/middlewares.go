package middlewares

import (
	"bytes"
	"compress/gzip"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"hash"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Allegathor/perfmon/internal/ciphers"
	"go.uber.org/zap"
)

type Flusher interface {
	Flush() error
}

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
	r.respData.code = code
	r.ResponseWriter.WriteHeader(code)
}

func CreateLogger(l *zap.SugaredLogger) func(http.Handler) http.Handler {

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
			l.Infoln(
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

// type gzipWriter struct {
// 	http.ResponseWriter
// 	writer       *gzip.Writer
// 	compressible *bool
// }

// func NewGzipWriter(rw http.ResponseWriter) *gzipWriter {
// 	return &gzipWriter{
// 		ResponseWriter: rw,
// 		writer:         nil,
// 		compressible:   nil,
// 	}
// }

// func (gw *gzipWriter) Write(p []byte) (int, error) {
// 	if gw.compressible == nil {
// 		var err error
// 		ct := gw.Header().Get("Content-Type")
// 		allowed := strings.HasPrefix(ct, "application/json") || strings.HasPrefix(ct, "text/html")
// 		gw.compressible = &allowed
// 		if *gw.compressible {
// 			gw.Header().Set("Content-Encoding", "gzip")
// 			gw.writer, err = gzip.NewWriterLevel(gw.ResponseWriter, gzip.BestSpeed)
// 			if err != nil {
// 				return 0, err
// 			}
// 			return gw.writer.Write(p)
// 		}

// 		return gw.ResponseWriter.Write(p)

// 	} else if *gw.compressible {
// 		return gw.writer.Write(p)
// 	}

// 	return gw.ResponseWriter.Write(p)
// }

// func (gw *gzipWriter) Close() error {
// 	if gw.writer != nil {
// 		return gw.writer.Close()
// 	}
// 	return nil
// }

var gzipWriterPool = &sync.Pool{
	New: func() any {
		w, _ := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
		return w
	},
}

type gzipWriter struct {
	http.ResponseWriter
	writer      *gzip.Writer
	pool        *sync.Pool
	wroteHeader bool
}

func NewGzipWriter(rw http.ResponseWriter) *gzipWriter {
	gz := gzipWriterPool.Get().(*gzip.Writer)
	gz.Reset(rw)

	return &gzipWriter{
		ResponseWriter: rw,
		writer:         gz,
		pool:           gzipWriterPool,
	}
}

func (gw *gzipWriter) GetWriter() io.Writer {
	return gw.writer
}

func (gw *gzipWriter) WriteHeader(code int) {
	if gw.wroteHeader {
		gw.ResponseWriter.WriteHeader(code)
		return
	}
	gw.wroteHeader = true
	defer gw.ResponseWriter.WriteHeader(code)

	if gw.Header().Get("Content-Encoding") != "" {
		return
	}

	gw.Header().Set("Content-Encoding", "gzip")
	gw.Header().Del("Content-Length")
}

func (gw *gzipWriter) Write(p []byte) (int, error) {
	if !gw.wroteHeader {
		gw.WriteHeader(http.StatusOK)
	}

	return gw.writer.Write(p)
}

func (gw *gzipWriter) Close() error {
	err := gw.writer.Close()
	gw.writer.Reset(io.Discard)
	gw.pool.Put(gw.writer)
	return err
}

func (gw *gzipWriter) Flush() {
	if f, ok := gw.GetWriter().(http.Flusher); ok {
		f.Flush()
	}

	if f, ok := gw.GetWriter().(Flusher); ok {
		f.Flush()
		if f, ok := gw.ResponseWriter.(Flusher); ok {
			f.Flush()
		}
	}
}

func CreateCompress(l *zap.SugaredLogger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			if !strings.Contains(req.Header.Get("Accept-Encoding"), "gzip") &&
				!strings.HasPrefix(req.Header.Get("Content-Type"), "application/json") &&
				!strings.HasPrefix(req.Header.Get("Content-Type"), "text/html") {
				next.ServeHTTP(rw, req)
				return
			}

			gw := NewGzipWriter(rw)
			defer func() {
				if closeErr := gw.Close(); closeErr != nil {
					l.Errorf("error closing writer in compress middleware: %s", closeErr)
					http.Error(rw, "decompression failed", http.StatusInternalServerError)
					return
				}
			}()

			next.ServeHTTP(gw, req)
		})
	}
}

func CreateUncompressReq(l *zap.SugaredLogger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			if strings.Contains(req.Header.Get("Content-Encoding"), "gzip") {
				gr, err := NewGzipReader(req.Body)
				if err != nil {
					l.Errorf("error creating reader in uncompress middleware: %s", err)
					http.Error(rw, "decompression failed", http.StatusInternalServerError)
					return
				}

				req.Body = gr
			}

			next.ServeHTTP(rw, req)
		})
	}
}

type SignWriter struct {
	http.ResponseWriter
	h hash.Hash
}

func NewSignWriter(rw http.ResponseWriter, key string) *SignWriter {
	return &SignWriter{
		ResponseWriter: rw,
		h:              hmac.New(sha256.New, []byte(key)),
	}
}

func (sw *SignWriter) Write(p []byte) (int, error) {
	if sw.h != nil {
		sw.Write(p)
		signStr := base64.URLEncoding.EncodeToString(sw.h.Sum(nil))
		sw.ResponseWriter.Header().Set("HashSHA256", signStr)
	}

	return sw.ResponseWriter.Write(p)
}

func CreateSumChecker(key string, l *zap.SugaredLogger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			hashHeader := req.Header.Get("HashSHA256")
			if hashHeader == "" {
				l.Warnln("HashSHA256 header is missed")
				next.ServeHTTP(rw, req)
				return
			}

			reqSign, err := base64.URLEncoding.DecodeString(hashHeader)
			if err != nil {
				l.Errorf("error encoding hash string: %s", err)
				http.Error(rw, "encoding error", http.StatusInternalServerError)
				return
			}

			var bodyBuf bytes.Buffer
			req.Body = io.NopCloser(io.TeeReader(req.Body, &bodyBuf))

			h := hmac.New(sha256.New, []byte(key))
			io.Copy(h, req.Body)
			sign := h.Sum(nil)
			fmt.Println(len(bodyBuf.Bytes()))

			if hmac.Equal(reqSign, sign) {
				req.Body = io.NopCloser(&bodyBuf)
				next.ServeHTTP(rw, req)
			} else {
				l.Errorln("signs are not equal")
				http.Error(rw, "invalid request", http.StatusBadRequest)
			}
		})
	}
}

func CreateSigner(key string, l *zap.SugaredLogger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			sw := NewSignWriter(rw, key)
			next.ServeHTTP(sw, req)
		})
	}
}

func CreateMsgDecrypter(key *rsa.PrivateKey, l *zap.SugaredLogger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			encBody, err := io.ReadAll(req.Body)
			if err != nil {
				l.Errorln("error reading body")
				http.Error(rw, "error reading body", http.StatusInternalServerError)
			}

			body, err := ciphers.DecryptMsg(key, encBody)
			if err != nil {
				l.Errorln("error decrypting body")
				http.Error(rw, "internal server error", http.StatusInternalServerError)
			}

			req.Body = io.NopCloser(bytes.NewReader(body))
			req.ContentLength = int64(len(body))
			next.ServeHTTP(rw, req)
		})
	}
}
