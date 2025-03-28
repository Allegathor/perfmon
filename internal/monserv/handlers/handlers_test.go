package handlers

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/Allegathor/perfmon/internal/repo"
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

func TestCreateUpdateHandler(t *testing.T) {
	type want[T int64 | float64] struct {
		contentType string
		code        int
		errMsg      string
		key         string
		value       T
	}
	tests := []struct {
		name        string
		success     bool
		req         *http.Request
		gaugeRepo   *repo.MRepo[float64]
		counterRepo *repo.MRepo[int64]
		want        want[int64]
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
			gaugeRepo:   repo.NewMRepo[float64](),
			counterRepo: repo.NewMRepo[int64](),
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
			gaugeRepo: repo.NewMRepo[float64](),
			counterRepo: &repo.MRepo[int64]{
				Data: map[string]int64{
					"PollCount": 56,
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
			gaugeRepo:   repo.NewMRepo[float64](),
			counterRepo: repo.NewMRepo[int64](),
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
			gaugeRepo:   repo.NewMRepo[float64](),
			counterRepo: repo.NewMRepo[int64](),
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
			gaugeRepo:   repo.NewMRepo[float64](),
			counterRepo: repo.NewMRepo[int64](),
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
			gaugeRepo:   repo.NewMRepo[float64](),
			counterRepo: repo.NewMRepo[int64](),
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
			gaugeRepo:   repo.NewMRepo[float64](),
			counterRepo: repo.NewMRepo[int64](),
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
			h := CreateUpdateHandler(tt.gaugeRepo, tt.counterRepo)
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

			ch := make(chan int64)
			time.Sleep(100 * time.Millisecond)
			chm := make(chan map[string]int64)
			go tt.counterRepo.Read(func(t repo.Tx[int64]) error {
				v, _ := t.Get(tt.want.key)
				vm := t.GetAll()

				ch <- v
				chm <- vm
				return nil
			})

			currentValue := <-ch
			currentMap := <-chm

			if tt.success {
				assert.Contains(t, currentMap, tt.want.key)
				assert.Equal(t, tt.want.value, currentValue)
			}

		})
	}
}

func TestCreateUpdateRootHandler(t *testing.T) {
	type want[T int64 | float64] struct {
		contentType string
		code        int
		errMsg      string
		key         string
		value       T
	}
	tests := []struct {
		name        string
		success     bool
		req         *http.Request
		gaugeRepo   *repo.MRepo[float64]
		counterRepo *repo.MRepo[int64]
		want        want[int64]
	}{
		{
			name:    "positive test #1",
			success: true,
			req: WrapWithChiCtx(
				httptest.NewRequest("POST", "/update",
					bytes.NewBuffer([]byte(`{"id":"PollCount","type":"counter","delta":99}`))),
				nil),
			gaugeRepo:   repo.NewMRepo[float64](),
			counterRepo: repo.NewMRepo[int64](),
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
			gaugeRepo: repo.NewMRepo[float64](),
			counterRepo: &repo.MRepo[int64]{
				Data: map[string]int64{
					"PollCount": 101,
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
			name:        "negative test #1 (method not allowed)",
			success:     false,
			req:         WrapWithChiCtx(httptest.NewRequest("GET", "/update", nil), nil),
			gaugeRepo:   repo.NewMRepo[float64](),
			counterRepo: repo.NewMRepo[int64](),
			want: want[int64]{
				contentType: "",
				code:        405,
				key:         "PollCount",
				value:       0,
				errMsg:      "",
			},
		},
		{
			name:        "negative test #2 (not found)",
			success:     false,
			req:         WrapWithChiCtx(httptest.NewRequest("POST", "/updat", nil), nil),
			gaugeRepo:   repo.NewMRepo[float64](),
			counterRepo: repo.NewMRepo[int64](),
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
			gaugeRepo:   repo.NewMRepo[float64](),
			counterRepo: repo.NewMRepo[int64](),
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
			gaugeRepo:   repo.NewMRepo[float64](),
			counterRepo: repo.NewMRepo[int64](),
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
			gaugeRepo:   repo.NewMRepo[float64](),
			counterRepo: repo.NewMRepo[int64](),
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
			h := CreateUpdateRootHandler(tt.gaugeRepo, tt.counterRepo)
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

			ch := make(chan int64)
			time.Sleep(100 * time.Millisecond)
			chm := make(chan map[string]int64)
			go tt.counterRepo.Read(func(t repo.Tx[int64]) error {
				v, _ := t.Get(tt.want.key)
				vm := t.GetAll()

				ch <- v
				chm <- vm
				return nil
			})

			currentValue := <-ch
			currentMap := <-chm

			if tt.success {
				assert.Contains(t, currentMap, tt.want.key)
				assert.Equal(t, tt.want.value, currentValue)
			}

		})
	}
}

func TestCreateRootHandler(t *testing.T) {
	dir, _ := os.Getwd()
	type want struct {
		contentType string
		code        int
		errMsg      string
		name        string
	}
	tests := []struct {
		name        string
		filePath    string
		success     bool
		req         *http.Request
		gaugeRepo   *repo.MRepo[float64]
		counterRepo *repo.MRepo[int64]
		want        want
	}{
		{
			name:      "positive test #1",
			success:   true,
			req:       WrapWithChiCtx(httptest.NewRequest("GET", "/", nil), nil),
			filePath:  dir + "/../../../templates/index.html",
			gaugeRepo: repo.NewMRepo[float64](),
			counterRepo: &repo.MRepo[int64]{
				Data: map[string]int64{
					"PollCount": 1,
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
			name:        "positive test #2",
			success:     true,
			req:         WrapWithChiCtx(httptest.NewRequest("GET", "/", nil), nil),
			filePath:    dir + "/../../../templates/index.html",
			counterRepo: repo.NewMRepo[int64](),
			gaugeRepo: &repo.MRepo[float64]{
				Data: map[string]float64{
					"Alloc": 1030.0012,
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
			name:        "negative test #1",
			success:     false,
			req:         WrapWithChiCtx(httptest.NewRequest("GET", "/", nil), nil),
			counterRepo: repo.NewMRepo[int64](),
			gaugeRepo: &repo.MRepo[float64]{
				Data: map[string]float64{
					"Alloc": 1030.0012,
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

			h := CreateRootHandler(tt.gaugeRepo, tt.counterRepo, tt.filePath)
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

func TestCreateValueHandler(t *testing.T) {
	type want struct {
		contentType string
		code        int
		errMsg      string
		value       string
	}
	tests := []struct {
		name        string
		success     bool
		req         *http.Request
		gaugeRepo   *repo.MRepo[float64]
		counterRepo *repo.MRepo[int64]
		want        want
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
			gaugeRepo: repo.NewMRepo[float64](),
			counterRepo: &repo.MRepo[int64]{
				Data: map[string]int64{
					"PollCount": 2,
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
			counterRepo: repo.NewMRepo[int64](),
			gaugeRepo: &repo.MRepo[float64]{
				Data: map[string]float64{
					"TotalAlloc": 11.0451,
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
			counterRepo: repo.NewMRepo[int64](),
			gaugeRepo:   repo.NewMRepo[float64](),
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
			counterRepo: repo.NewMRepo[int64](),
			gaugeRepo:   repo.NewMRepo[float64](),
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
			counterRepo: repo.NewMRepo[int64](),
			gaugeRepo:   repo.NewMRepo[float64](),
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

			h := CreateValueHandler(tt.gaugeRepo, tt.counterRepo)
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

func TestCreateValueRootHandler(t *testing.T) {
	type want struct {
		contentType string
		code        int
		errMsg      string
		respBody    string
	}
	tests := []struct {
		name        string
		success     bool
		req         *http.Request
		gaugeRepo   *repo.MRepo[float64]
		counterRepo *repo.MRepo[int64]
		want        want
	}{
		{
			name:    "positive test #1",
			success: true,
			req: WrapWithChiCtx(
				httptest.NewRequest("POST", "/value",
					bytes.NewBuffer([]byte(`{"id":"PollCount","type":"counter"}`))),
				nil),
			gaugeRepo: repo.NewMRepo[float64](),
			counterRepo: &repo.MRepo[int64]{
				Data: map[string]int64{
					"PollCount": 64,
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
			counterRepo: repo.NewMRepo[int64](),
			gaugeRepo: &repo.MRepo[float64]{
				Data: map[string]float64{
					"PollCount": 64,
					"Alloc":     15994.03143,
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
			name:        "negative test #1 (method not allowed)",
			success:     false,
			req:         WrapWithChiCtx(httptest.NewRequest("GET", "/value", nil), nil),
			counterRepo: repo.NewMRepo[int64](),
			gaugeRepo:   repo.NewMRepo[float64](),
			want: want{
				contentType: "",
				code:        405,
				respBody:    "",
				errMsg:      "",
			},
		},
		{
			name:        "negative test #2 (not found)",
			success:     false,
			req:         WrapWithChiCtx(httptest.NewRequest("POST", "/valu", nil), nil),
			counterRepo: repo.NewMRepo[int64](),
			gaugeRepo:   repo.NewMRepo[float64](),
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
			counterRepo: repo.NewMRepo[int64](),
			gaugeRepo:   repo.NewMRepo[float64](),
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
			counterRepo: repo.NewMRepo[int64](),
			gaugeRepo:   repo.NewMRepo[float64](),
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
			counterRepo: repo.NewMRepo[int64](),
			gaugeRepo:   repo.NewMRepo[float64](),
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
			h := CreateValueRootHandler(tt.gaugeRepo, tt.counterRepo)
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
