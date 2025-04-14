package handlers

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Allegathor/perfmon/internal/mondata"
	"github.com/Allegathor/perfmon/internal/repo/memory"
	"github.com/Allegathor/perfmon/internal/repo/safe"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func WrapWithChiCtx(req *http.Request, params map[string]string) *http.Request {
	if req.RequestURI == "/update" || req.RequestURI == "/value" {
		req.Header.Add("Content-Type", "application/json")
	}

	ctx := chi.NewRouteContext()
	for k, v := range params {
		ctx.URLParams.Add(k, v)
	}

	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))
}

type logger struct{}

func (l *logger) Errorln(...any) {}

// MARK: Update
func TestUpdateHandler(t *testing.T) {
	type want[T int64 | float64] struct {
		contentType string
		code        int
		errMsg      string
		key         string
		value       T
	}
	tests := []struct {
		name    string
		success bool
		req     *http.Request
		db      *memory.MemorySt
		want    want[int64]
	}{
		{
			name:    "positive test #1",
			success: true,
			req: WrapWithChiCtx(
				httptest.NewRequest("POST", "/update/counter/PollCount/1", nil), map[string]string{
					"type":  "counter",
					"name":  "PollCount",
					"value": "1",
				},
			),
			db: memory.InitEmpty(),
			want: want[int64]{
				contentType: "",
				code:        200,
				key:         "PollCount",
				value:       1,
				errMsg:      "",
			},
		},
		{
			name:    "positive test #2",
			success: true,
			req: WrapWithChiCtx(
				httptest.NewRequest("POST", "/update/counter/PollCount/21", nil), map[string]string{
					"type":  "counter",
					"name":  "PollCount",
					"value": "2",
				},
			),
			db: &memory.MemorySt{
				Gauge: &safe.MRepo[mondata.GaugeVType]{},
				Counter: &safe.MRepo[mondata.CounterVType]{
					Data: mondata.CounterMap{"PollCount": 56},
				},
			},
			want: want[int64]{
				contentType: "",
				code:        200,
				key:         "PollCount",
				value:       77,
				errMsg:      "",
			},
		},
		{
			name:    "negative test #1 (method not allowed)",
			success: false,
			req: WrapWithChiCtx(
				httptest.NewRequest("GET", "/update/counter/PollCount/21", nil), map[string]string{
					"type":  "counter",
					"name":  "PollCount",
					"value": "21",
				},
			),
			db: memory.InitEmpty(),
			want: want[int64]{
				contentType: "",
				code:        405,
				key:         "PollCount",
				value:       21,
				errMsg:      "",
			},
		},
		{
			name:    "negative test #2 (not found)",
			success: false,
			req: WrapWithChiCtx(
				httptest.NewRequest("POST", "/updat/counter/PollCount/1", nil), map[string]string{
					"type":  "counter",
					"name":  "PollCount",
					"value": "1",
				},
			),
			db: memory.InitEmpty(),
			want: want[int64]{
				contentType: "text/plain; charset=utf-8",
				code:        404,
				key:         "PollCount",
				value:       1,
				errMsg:      "",
			},
		},
		{
			name:    "negative test #3 (missing name)",
			success: false,
			req: WrapWithChiCtx(
				httptest.NewRequest("POST", "/update/counter//1", nil), map[string]string{
					"type":  "counter",
					"name":  "",
					"value": "1",
				},
			),
			db: memory.InitEmpty(),
			want: want[int64]{
				contentType: "text/plain; charset=utf-8",
				code:        404,
				key:         "PollCount",
				value:       1,
				errMsg:      "",
			},
		},
		{
			name:    "negative test #4 (invalid value)",
			success: false,
			req: WrapWithChiCtx(
				httptest.NewRequest("POST", "/update/counter/PollCount/value", nil), map[string]string{
					"type":  "counter",
					"name":  "PollCount",
					"value": "value",
				},
			),
			db: memory.InitEmpty(),
			want: want[int64]{
				contentType: "text/plain; charset=utf-8",
				code:        400,
				key:         "PollCount",
				value:       1,
				errMsg:      "",
			},
		},
		{
			name:    "negative test #5 (incorrect type)",
			success: false,
			req: WrapWithChiCtx(
				httptest.NewRequest("POST", "/update/type/PollCount/value", nil), map[string]string{
					"type":  "type",
					"name":  "PollCount",
					"value": "value",
				},
			),
			db: memory.InitEmpty(),
			want: want[int64]{
				contentType: "text/plain; charset=utf-8",
				code:        400,
				key:         "PollCount",
				value:       1,
				errMsg:      "",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewAPI(tt.db, &logger{}).UpdateHandler
			r := chi.NewRouter()
			r.Post("/update/{type}/{name}/{value}", h)
			recorder := httptest.NewRecorder()
			r.ServeHTTP(recorder, tt.req)

			res := recorder.Result()
			assert.Equal(t, tt.want.code, res.StatusCode)
			assert.Equal(t, tt.want.contentType, res.Header.Get("Content-Type"))

			_, err := io.ReadAll(res.Body)
			require.NoError(t, err)

			err = res.Body.Close()
			require.NoError(t, err)

			currentValue, _, _ := tt.db.GetCounter(context.TODO(), tt.want.key)
			currentMap, _ := tt.db.GetCounterAll(context.TODO())

			if tt.success {
				assert.Contains(t, currentMap, tt.want.key)
				assert.Equal(t, tt.want.value, currentValue)
			}

		})
	}
}

// MARK: Update root

func TestUpdateRootHandler(t *testing.T) {
	type want[T int64 | float64] struct {
		contentType string
		code        int
		errMsg      string
		key         string
		value       T
	}
	tests := []struct {
		name    string
		success bool
		req     *http.Request
		db      *memory.MemorySt
		want    want[int64]
	}{
		{
			name:    "positive test #1",
			success: true,
			req: WrapWithChiCtx(
				httptest.NewRequest("POST", "/update",
					bytes.NewBuffer([]byte(`{"id":"PollCount","type":"counter","delta":99}`))),
				nil),
			db: memory.InitEmpty(),
			want: want[int64]{
				contentType: "",
				code:        200,
				key:         "PollCount",
				value:       99,
				errMsg:      "",
			},
		},
		{
			name:    "positive test #2",
			success: true,
			req: WrapWithChiCtx(
				httptest.NewRequest("POST", "/update",
					bytes.NewBuffer([]byte(`{"id":"PollCount","type":"counter","delta":50}`))),
				nil),
			db: &memory.MemorySt{
				Gauge: &safe.MRepo[mondata.GaugeVType]{},
				Counter: &safe.MRepo[mondata.CounterVType]{
					Data: mondata.CounterMap{
						"PollCount": 101,
					},
				},
			},
			want: want[int64]{
				contentType: "",
				code:        200,
				key:         "PollCount",
				value:       151,
				errMsg:      "",
			},
		},
		{
			name:    "negative test #1 (method not allowed)",
			success: false,
			req:     WrapWithChiCtx(httptest.NewRequest("GET", "/update", nil), nil),
			db:      memory.InitEmpty(),
			want: want[int64]{
				contentType: "",
				code:        405,
				key:         "PollCount",
				value:       0,
				errMsg:      "",
			},
		},
		{
			name:    "negative test #2 (not found)",
			success: false,
			req:     WrapWithChiCtx(httptest.NewRequest("POST", "/updat", nil), nil),
			db:      memory.InitEmpty(),
			want: want[int64]{
				contentType: "text/plain; charset=utf-8",
				code:        404,
				key:         "PollCount",
				value:       0,
				errMsg:      "",
			},
		},
		{
			name:    "negative test #3 (missing name)",
			success: false,
			req: WrapWithChiCtx(
				httptest.NewRequest("POST", "/update",
					bytes.NewBuffer([]byte(`{"id":"","type":"counter","delta":50}`))),
				nil),
			db: memory.InitEmpty(),
			want: want[int64]{
				contentType: "text/plain; charset=utf-8",
				code:        404,
				key:         "",
				value:       0,
				errMsg:      "",
			},
		},
		{
			name:    "negative test #4 (invalid value)",
			success: false,
			req: WrapWithChiCtx(
				httptest.NewRequest("POST", "/update",
					bytes.NewBuffer([]byte(`{"id":"PollCount","type":"counter","value":"one"}`))),
				nil),
			db: memory.InitEmpty(),
			want: want[int64]{
				contentType: "text/plain; charset=utf-8",
				code:        400,
				key:         "PollCount",
				value:       0,
				errMsg:      "",
			},
		},
		{
			name:    "negative test #5 (incorrect type)",
			success: false,
			req: WrapWithChiCtx(
				httptest.NewRequest("POST", "/update",
					bytes.NewBuffer([]byte(`{"id":"PollCount","type":"sometype","value":1}`))),
				nil),
			db: memory.InitEmpty(),
			want: want[int64]{
				contentType: "text/plain; charset=utf-8",
				code:        400,
				key:         "PollCount",
				value:       1,
				errMsg:      "",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewAPI(tt.db, &logger{}).UpdateRootHandler
			r := chi.NewRouter()
			r.Post("/update", h)
			recorder := httptest.NewRecorder()
			r.ServeHTTP(recorder, tt.req)

			res := recorder.Result()
			assert.Equal(t, tt.want.code, res.StatusCode)
			assert.Equal(t, tt.want.contentType, res.Header.Get("Content-Type"))

			_, err := io.ReadAll(res.Body)
			require.NoError(t, err)

			err = res.Body.Close()
			require.NoError(t, err)

			currentValue, _, _ := tt.db.GetCounter(context.TODO(), tt.want.key)
			currentMap, _ := tt.db.GetCounterAll(context.TODO())

			if tt.success {
				assert.Contains(t, currentMap, tt.want.key)
				assert.Equal(t, tt.want.value, currentValue)
			}

		})
	}
}

// MARK: Root
func TestCreateRootHandler(t *testing.T) {
	dir, _ := os.Getwd()
	type want struct {
		contentType string
		code        int
		errMsg      string
		name        string
	}
	tests := []struct {
		name     string
		filePath string
		success  bool
		req      *http.Request
		db       *memory.MemorySt
		want     want
	}{
		{
			name:     "positive test #1",
			success:  true,
			req:      WrapWithChiCtx(httptest.NewRequest("GET", "/", nil), nil),
			filePath: dir + "/../../../templates/index.html",
			db: &memory.MemorySt{
				Gauge: &safe.MRepo[mondata.GaugeVType]{},
				Counter: &safe.MRepo[mondata.CounterVType]{
					Data: mondata.CounterMap{
						"PollCount": 1,
					},
				},
			},
			want: want{
				contentType: "text/html; charset=utf-8",
				code:        200,
				errMsg:      "",
				name:        "PollCount",
			},
		},
		{
			name:     "positive test #2",
			success:  true,
			req:      WrapWithChiCtx(httptest.NewRequest("GET", "/", nil), nil),
			filePath: dir + "/../../../templates/index.html",
			db: &memory.MemorySt{
				Counter: &safe.MRepo[mondata.CounterVType]{},
				Gauge: &safe.MRepo[mondata.GaugeVType]{
					Data: mondata.GaugeMap{
						"Alloc": 1030.0012,
					},
				},
			},
			want: want{
				contentType: "text/html; charset=utf-8",
				code:        200,
				errMsg:      "",
				name:        "Alloc",
			},
		},
		{
			name:    "negative test #1",
			success: false,
			req:     WrapWithChiCtx(httptest.NewRequest("GET", "/", nil), nil),
			db: &memory.MemorySt{
				Counter: &safe.MRepo[mondata.CounterVType]{},
				Gauge: &safe.MRepo[mondata.GaugeVType]{
					Data: mondata.GaugeMap{
						"Alloc": 1030.0012,
					},
				},
			},
			want: want{
				contentType: "text/plain; charset=utf-8",
				code:        500,
				errMsg:      "",
				name:        "Alloc",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			h := NewAPI(tt.db, &logger{}).CreateRootHandler(tt.filePath)
			r := chi.NewRouter()
			r.Get("/", h)

			recorder := httptest.NewRecorder()
			r.ServeHTTP(recorder, tt.req)

			res := recorder.Result()
			assert.Equal(t, tt.want.code, res.StatusCode)
			assert.Equal(t, tt.want.contentType, res.Header.Get("Content-Type"))

			respBody, err := io.ReadAll(res.Body)
			require.NoError(t, err)

			err = res.Body.Close()
			require.NoError(t, err)

			if tt.success {
				assert.Contains(t, string(respBody), tt.want.name)
			}

		})
	}
}

// MARK: Value
func TestValueHandler(t *testing.T) {
	type want struct {
		contentType string
		code        int
		errMsg      string
		value       string
	}
	tests := []struct {
		name    string
		success bool
		req     *http.Request
		db      *memory.MemorySt
		want    want
	}{
		{
			name:    "positive test #1",
			success: true,
			req: WrapWithChiCtx(
				httptest.NewRequest("GET", "/value/counter/PollCount", nil), map[string]string{
					"type": "counter",
					"name": "PollCount",
				},
			),
			db: &memory.MemorySt{
				Gauge: &safe.MRepo[mondata.GaugeVType]{},
				Counter: &safe.MRepo[mondata.CounterVType]{
					Data: mondata.CounterMap{
						"PollCount": 2,
					},
				},
			},
			want: want{
				contentType: "text/plain; charset=utf-8",
				code:        200,
				errMsg:      "",
				value:       "2",
			},
		},
		{
			name:    "positive test #2",
			success: true,
			req: WrapWithChiCtx(
				httptest.NewRequest("GET", "/value/gauge/TotalAlloc", nil), map[string]string{
					"type": "gauge",
					"name": "TotalAlloc",
				},
			),
			db: &memory.MemorySt{
				Gauge: &safe.MRepo[mondata.GaugeVType]{
					Data: mondata.GaugeMap{
						"TotalAlloc": 11.0451,
					},
				},
				Counter: &safe.MRepo[mondata.CounterVType]{
					Data: mondata.CounterMap{},
				},
			},
			want: want{
				contentType: "text/plain; charset=utf-8",
				code:        200,
				errMsg:      "",
				value:       "11.0451",
			},
		},
		{
			name:    "negative test #1 (wrong type, bad req)",
			success: false,
			req: WrapWithChiCtx(
				httptest.NewRequest("GET", "/value/gaug/TotalAlloc", nil), map[string]string{
					"type": "gaug",
					"name": "TotalAlloc",
				},
			),
			db: memory.InitEmpty(),
			want: want{
				contentType: "text/plain; charset=utf-8",
				code:        400,
				errMsg:      "",
				value:       "",
			},
		},
		{
			name:    "negative test #2 (value doesn't exist)",
			success: false,
			req: WrapWithChiCtx(
				httptest.NewRequest("GET", "/value/gauge/TotalAlloc", nil), map[string]string{
					"type": "gaug",
					"name": "TotalAlloc",
				},
			),
			db: memory.InitEmpty(),
			want: want{
				contentType: "text/plain; charset=utf-8",
				code:        404,
				errMsg:      "",
				value:       "",
			},
		},
		{
			name:    "negative test #3 (empty name)",
			success: false,
			req: WrapWithChiCtx(
				httptest.NewRequest("GET", "/value/gauge/", nil), map[string]string{
					"type": "gauge",
					"name": "",
				},
			),
			db: memory.InitEmpty(),
			want: want{
				contentType: "text/plain; charset=utf-8",
				code:        404,
				errMsg:      "",
				value:       "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewAPI(tt.db, &logger{}).ValueHandler
			r := chi.NewRouter()
			r.Get("/value/{type}/{name}", h)
			recorder := httptest.NewRecorder()

			r.ServeHTTP(recorder, tt.req)

			res := recorder.Result()
			assert.Equal(t, tt.want.code, res.StatusCode)
			assert.Equal(t, tt.want.contentType, res.Header.Get("Content-Type"))

			respBody, err := io.ReadAll(res.Body)
			require.NoError(t, err)

			err = res.Body.Close()
			require.NoError(t, err)

			if tt.success {
				assert.Contains(t, string(respBody), tt.want.value)
			}

		})
	}
}

func TestValueRootHandler(t *testing.T) {
	type want struct {
		contentType string
		code        int
		errMsg      string
		respBody    string
	}
	tests := []struct {
		name    string
		success bool
		req     *http.Request
		db      *memory.MemorySt
		want    want
	}{
		{
			name:    "positive test #1",
			success: true,
			req: WrapWithChiCtx(
				httptest.NewRequest("POST", "/value",
					bytes.NewBuffer([]byte(`{"id":"PollCount","type":"counter"}`))),
				nil),
			db: &memory.MemorySt{
				Gauge: &safe.MRepo[mondata.GaugeVType]{},
				Counter: &safe.MRepo[mondata.CounterVType]{
					Data: mondata.CounterMap{
						"PollCount": 64,
					},
				},
			},
			want: want{
				contentType: "application/json; charset=utf-8",
				code:        200,
				respBody:    `{"id":"PollCount","type":"counter","delta":64}`,
				errMsg:      "",
			},
		},
		{
			name:    "positive test #2",
			success: true,
			req: WrapWithChiCtx(
				httptest.NewRequest("POST", "/value",
					bytes.NewBuffer([]byte(`{"id":"Alloc","type":"gauge"}`))),
				nil),
			db: &memory.MemorySt{
				Gauge: &safe.MRepo[mondata.GaugeVType]{
					Data: mondata.GaugeMap{
						"Alloc": 15994.03143,
					},
				},
				Counter: &safe.MRepo[mondata.CounterVType]{
					Data: mondata.CounterMap{},
				},
			},
			want: want{
				contentType: "application/json; charset=utf-8",
				code:        200,
				respBody:    `{"id":"Alloc","type":"gauge","value":15994.03143}`,
				errMsg:      "",
			},
		},
		{
			name:    "negative test #1 (method not allowed)",
			success: false,
			req:     WrapWithChiCtx(httptest.NewRequest("GET", "/value", nil), nil),
			db:      memory.InitEmpty(),
			want: want{
				contentType: "",
				code:        405,
				respBody:    "",
				errMsg:      "",
			},
		},
		{
			name:    "negative test #2 (not found)",
			success: false,
			req:     WrapWithChiCtx(httptest.NewRequest("POST", "/valu", nil), nil),
			db:      memory.InitEmpty(),
			want: want{
				contentType: "text/plain; charset=utf-8",
				code:        404,
				respBody:    "",
				errMsg:      "",
			},
		},
		{
			name:    "negative test #3 (missing name)",
			success: false,
			req: WrapWithChiCtx(
				httptest.NewRequest("POST", "/value",
					bytes.NewBuffer([]byte(`{"id":"","type":"counter"}`))),
				nil),
			db: memory.InitEmpty(),
			want: want{
				contentType: "text/plain; charset=utf-8",
				code:        404,
				respBody:    "",
				errMsg:      "",
			},
		},
		{
			name:    "negative test #4 (not found: no such key in storage)",
			success: false,
			req: WrapWithChiCtx(
				httptest.NewRequest("POST", "/value",
					bytes.NewBuffer([]byte(`{"id":"PollCoutn","type":"counter"}`))),
				nil),
			db: memory.InitEmpty(),
			want: want{
				contentType: "text/plain; charset=utf-8",
				code:        404,
				respBody:    "",
				errMsg:      "",
			},
		},
		{
			name:    "negative test #5 (incorrect type)",
			success: false,
			req: WrapWithChiCtx(
				httptest.NewRequest("POST", "/value",
					bytes.NewBuffer([]byte(`{"id":"PollCount","type":"sometype"}`))),
				nil),
			db: memory.InitEmpty(),
			want: want{
				contentType: "text/plain; charset=utf-8",
				code:        400,
				respBody:    "",
				errMsg:      "",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewAPI(tt.db, &logger{}).ValueRootHandler
			r := chi.NewRouter()
			r.Post("/value", h)
			recorder := httptest.NewRecorder()
			r.ServeHTTP(recorder, tt.req)

			res := recorder.Result()
			assert.Equal(t, tt.want.code, res.StatusCode)
			assert.Equal(t, tt.want.contentType, res.Header.Get("Content-Type"))

			respBody, err := io.ReadAll(res.Body)
			require.NoError(t, err)

			err = res.Body.Close()
			require.NoError(t, err)

			if tt.success {
				assert.Equal(t, tt.want.respBody, string(respBody))
			}

		})
	}
}
