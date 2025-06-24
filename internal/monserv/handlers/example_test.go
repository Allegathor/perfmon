package handlers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http/httptest"
	"os"

	"github.com/Allegathor/perfmon/internal/mondata"
	"github.com/Allegathor/perfmon/internal/repo/memory"
	"github.com/Allegathor/perfmon/internal/repo/safe"
	"github.com/go-chi/chi/v5"
)

func ExampleUpdateHandler() {
	db := memory.InitEmpty()
	req := WrapWithChiCtx(
		httptest.NewRequest("POST", "/update/counter/PollCount/1", nil), map[string]string{
			"type":  "counter",
			"name":  "PollCount",
			"value": "1",
		},
	)

	h := NewAPI(db, &ErrLoggerMock{}).UpdateHandler
	r := chi.NewRouter()
	r.Post("/update/{type}/{name}/{value}", h)
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, req)

	res := recorder.Result()
	out1 := res.StatusCode
	fmt.Println(out1)
	out2, _, _ := db.GetCounter(context.TODO(), "PollCount")
	fmt.Println(out2)

	// Output:
	// 200
	// 1
}

func ExampleUpdateRootHandler() {
	db := memory.InitEmpty()

	req := WrapWithChiCtx(
		httptest.NewRequest("POST", "/update",
			bytes.NewBuffer([]byte(`{"id":"PollCount","type":"counter","delta":99}`))),
		nil)

	h := NewAPI(db, &ErrLoggerMock{}).UpdateRootHandler
	r := chi.NewRouter()
	r.Post("/update", h)
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, req)

	res := recorder.Result()
	out1 := res.StatusCode
	fmt.Println(out1)
	out2, _, _ := db.GetCounter(context.TODO(), "PollCount")
	fmt.Println(out2)

	// Output:
	// 200
	// 99
}

func ExampleCreateRootHandler() {
	dir, _ := os.Getwd()
	filePath := dir + "/../../../templates/index.html"

	db := memory.InitEmpty()
	req := WrapWithChiCtx(httptest.NewRequest("GET", "/", nil), nil)

	h := NewAPI(db, &ErrLoggerMock{}).CreateRootHandler(filePath)
	r := chi.NewRouter()
	r.Get("/", h)

	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, req)

	res := recorder.Result()
	out1 := res.StatusCode
	fmt.Println(out1)
	out2 := res.Header.Get("Content-type")
	fmt.Println(out2)

	// Output:
	// 200
	// text/html; charset=utf-8
}

func ExampleValueHandler() {
	db := &memory.MemorySt{
		Gauge: &safe.MRepo[mondata.GaugeVType]{},
		Counter: &safe.MRepo[mondata.CounterVType]{
			Data: mondata.CounterMap{
				"PollCount": 2,
			},
		},
	}
	req := WrapWithChiCtx(
		httptest.NewRequest("GET", "/value/counter/PollCount", nil), map[string]string{
			"type": "counter",
			"name": "PollCount",
		},
	)

	h := NewAPI(db, &ErrLoggerMock{}).ValueHandler
	r := chi.NewRouter()
	r.Get("/value/{type}/{name}", h)
	recorder := httptest.NewRecorder()

	r.ServeHTTP(recorder, req)

	res := recorder.Result()
	out1 := res.StatusCode
	fmt.Println(out1)
	out2, _, _ := db.GetCounter(context.TODO(), "PollCount")
	fmt.Println(out2)

	// Output:
	// 200
	// 2
}

func ExampleValueRootHandler() {
	db := &memory.MemorySt{
		Gauge: &safe.MRepo[mondata.GaugeVType]{},
		Counter: &safe.MRepo[mondata.CounterVType]{
			Data: mondata.CounterMap{
				"PollCount": 64,
			},
		},
	}
	req := WrapWithChiCtx(
		httptest.NewRequest("POST", "/value",
			bytes.NewBuffer([]byte(`{"id":"PollCount","type":"counter"}`))),
		nil)

	h := NewAPI(db, &ErrLoggerMock{}).ValueRootHandler
	r := chi.NewRouter()
	r.Post("/value", h)
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, req)

	res := recorder.Result()

	out1 := res.StatusCode
	fmt.Println(out1)
	out2, _, _ := db.GetCounter(context.TODO(), "PollCount")
	fmt.Println(out2)
	respBody, _ := io.ReadAll(res.Body)
	out3 := string(respBody)
	fmt.Println(out3)
	res.Body.Close()

	// Output:
	// 200
	// 64
	// {"id":"PollCount","type":"counter","delta":64}
}
