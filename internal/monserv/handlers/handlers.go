package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strings"

	"github.com/Allegathor/perfmon/internal/mondata"
	pb "github.com/Allegathor/perfmon/internal/proto"
	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	URLPathType  = "type"
	URLPathName  = "name"
	URLPathValue = "value"
)

// RespError is used for errors in handlers
type RespError struct {
	err error
	msg string // provide message for http-response
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

// Database interface
type MDB interface {
	Getters
	Setters
	Ping(ctx context.Context) error
}

type ErrLogger interface {
	Errorln(...any)
}

// HTTP API
type API struct {
	db     MDB
	logger ErrLogger
}

func NewAPI(db MDB, logger ErrLogger) *API {
	return &API{
		db,
		logger,
	}
}

// Logs error and responds with error code and error message
func (api *API) Error(rw http.ResponseWriter, err *RespError, code int) {
	api.logger.Errorln(err)
	http.Error(rw, err.Msg(), code)
}

// Responds with html-template which represents table with all collected metrics
func (api *API) CreateRootHandler(path string) http.HandlerFunc {

	if path == "" {
		dir, _ := os.Getwd()
		path = dir + "/templates/index.html"
	}

	tmpl, tmplErr := template.New("index.html").ParseFiles(path)

	return func(rw http.ResponseWriter, req *http.Request) {
		type Vals interface {
			map[string]float64 | map[string]int64
		}

		type Table[T Vals] struct {
			Name    string
			Content T
		}

		gVals, err := api.db.GetGaugeAll(req.Context())
		if err != nil {
			respErr := NewRespError("an error occured while acquaring gauge values from db", err)
			api.Error(rw, respErr, http.StatusInternalServerError)
			return
		}

		cVals, err := api.db.GetCounterAll(req.Context())
		if err != nil {
			respErr := NewRespError("an error occured while acquaring counter values from db", err)
			api.Error(rw, respErr, http.StatusInternalServerError)
			return
		}

		viewData := []any{
			Table[map[string]float64]{Name: "Gauge", Content: gVals},
			Table[map[string]int64]{Name: "Counter", Content: cVals},
		}

		if tmplErr != nil {
			respErr := NewRespError("file parsing error", err)
			api.Error(rw, respErr, http.StatusInternalServerError)
			return
		}

		rw.Header().Set("Content-Type", "text/html; charset=utf-8")
		err = tmpl.Execute(rw, viewData)
		if err != nil {
			respErr := NewRespError("template execution error", err)
			api.Error(rw, respErr, http.StatusInternalServerError)
		}
	}
}

// Validates metric type, name and value.
//
// If succeeded updates values in database.
func updateMetrics(ctx context.Context, m *mondata.Metrics, db MDB) (int, *RespError) {
	if m.ID == "" {
		return http.StatusNotFound, NewRespError("name must contain a value", nil)
	}

	switch m.MType {
	case mondata.GaugeType:
		if m.SValue != "" {
			v, err := mondata.ParseGauge(m.SValue)
			if err != nil {
				return http.StatusBadRequest, NewRespError("invalid value", err)
			}
			m.Value = &v
		}

		err := db.SetGauge(ctx, m.ID, *m.Value)
		if err != nil {
			return http.StatusInternalServerError, NewRespError("setting gauge value in db failed", err)
		}

		return http.StatusOK, nil
	case mondata.CounterType:
		if m.SValue != "" {
			d, err := mondata.ParseCounter(m.SValue)
			if err != nil {
				return http.StatusBadRequest, NewRespError("invalid value", err)
			}
			m.Delta = &d
		}

		err := db.SetCounter(ctx, m.ID, *m.Delta)
		if err != nil {
			return http.StatusInternalServerError, NewRespError("setting counter value in db failed", err)
		}
		return http.StatusOK, nil
	default:
		return http.StatusBadRequest, NewRespError("incorrect request type", nil)
	}
}

// Accepts requests with URL params: type/name/value.
//
// Updates specified metric and respond with 200 if succeeded.
func (api *API) UpdateHandler(rw http.ResponseWriter, req *http.Request) {
	var m = &mondata.Metrics{}

	m.MType = chi.URLParam(req, URLPathType)
	m.ID = chi.URLParam(req, URLPathName)
	m.SValue = chi.URLParam(req, URLPathValue)
	code, err := updateMetrics(req.Context(), m, api.db)
	if err != nil {
		api.Error(rw, err, code)
		return
	}

	rw.WriteHeader(code)
}

// Accepts requests with JSON-body, that contains single metric data.
//
// Updates specified metric and respond with 200 if succeeded.
func (api *API) UpdateRootHandler(rw http.ResponseWriter, req *http.Request) {
	if strings.Contains(req.Header.Get("Content-Type"), "application/json") {
		var buf bytes.Buffer

		_, err := buf.ReadFrom(req.Body)
		if err != nil {
			respErr := NewRespError("working with request body failed", err)
			api.Error(rw, respErr, http.StatusInternalServerError)
			return
		}

		defer func() {
			err := req.Body.Close()
			if err != nil {
				respErr := NewRespError("working with request body failed", err)
				api.Error(rw, respErr, http.StatusInternalServerError)
				return
			}
		}()

		m := &mondata.Metrics{}
		if err := json.Unmarshal(buf.Bytes(), m); err != nil {
			respErr := NewRespError("unmarshaling failed", err)
			api.Error(rw, respErr, http.StatusBadRequest)
			return
		}

		code, respErr := updateMetrics(req.Context(), m, api.db)
		if respErr != nil {
			api.Error(rw, respErr, code)
			return
		}

		rw.WriteHeader(code)
	} else {
		respErr := NewRespError("unsupported content type", nil)
		api.Error(rw, respErr, http.StatusBadRequest)
	}
}

// Accepts requests with JSON-body, that contains array of metric data.
//
// Updates specified metrics and respond with 200 if succeeded.
func (api *API) UpdateBatchHandler(rw http.ResponseWriter, req *http.Request) {
	if strings.Contains(req.Header.Get("Content-Type"), "application/json") {
		var buf bytes.Buffer

		_, err := buf.ReadFrom(req.Body)
		if err != nil {
			respErr := NewRespError("working with request body failed", err)
			api.Error(rw, respErr, http.StatusInternalServerError)
			return
		}

		defer func() {
			err := req.Body.Close()
			if err != nil {
				respErr := NewRespError("working with request body failed", err)
				api.Error(rw, respErr, http.StatusInternalServerError)
				return
			}
		}()

		mm := &[]mondata.Metrics{}
		if err := json.Unmarshal(buf.Bytes(), mm); err != nil {
			respErr := NewRespError("unmarshaling failed", err)
			api.Error(rw, respErr, http.StatusBadRequest)
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
				if cv, ok := cm[rec.ID]; ok {
					cm[rec.ID] = cv + *rec.Delta
					continue
				}
				cm[rec.ID] = *rec.Delta
			}
		}

		if len(gm) == 0 && len(cm) == 0 {
			respErr := NewRespError("nothing to update", nil)
			api.Error(rw, respErr, http.StatusBadRequest)
			return
		}

		if len(gm) > 0 {
			if err := api.db.SetGaugeAll(req.Context(), gm); err != nil {
				respErr := NewRespError("gauge batch update to db failed", err)
				api.Error(rw, respErr, http.StatusInternalServerError)
				return
			}
		}

		if len(cm) > 0 {
			if err := api.db.SetCounterAll(req.Context(), cm); err != nil {
				respErr := NewRespError("counter batch update to db failed", err)
				api.Error(rw, respErr, http.StatusInternalServerError)
				return
			}
		}

		rw.WriteHeader(http.StatusOK)

	} else {
		respErr := NewRespError("unsupported content type", nil)
		api.Error(rw, respErr, http.StatusBadRequest)
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

	switch m.MType {
	case mondata.GaugeType:
		v, ok, err := db.GetGauge(ctx, m.ID)
		if err != nil {
			return &vhData{code: http.StatusInternalServerError}, NewRespError("getting gauge value from db failed", err)
		} else if ok {
			return &vhData{
				code: http.StatusOK,
				metrics: &mondata.Metrics{
					ID: m.ID, MType: m.MType, Value: &v, SValue: mondata.FormatGauge(v),
				},
			}, nil
		}
		return &vhData{code: http.StatusNotFound}, NewRespError("value doesn't exist in the storage", nil)
	case mondata.CounterType:
		v, ok, err := db.GetCounter(ctx, m.ID)
		if err != nil {
			return &vhData{code: http.StatusInternalServerError}, NewRespError("getting counter value from db failed", err)
		} else if ok {
			return &vhData{
				code: http.StatusOK,
				metrics: &mondata.Metrics{
					ID: m.ID, MType: m.MType, Delta: &v, SValue: mondata.FormatCounter(v),
				},
			}, nil
		}
		return &vhData{code: http.StatusNotFound}, NewRespError("value doesn't exist in the storage", nil)
	default:
		return &vhData{code: http.StatusBadRequest}, NewRespError("incorrect request type", nil)
	}
}

// Accepts request with next URL params: type/name.
//
// Responds with text/plain body containing specified value.
func (api *API) ValueHandler(rw http.ResponseWriter, req *http.Request) {
	m := &mondata.Metrics{}
	m.MType = chi.URLParam(req, URLPathType)
	m.ID = chi.URLParam(req, URLPathName)
	vhd, respErr := getVhData(req.Context(), m, api.db)
	if respErr != nil {
		api.Error(rw, respErr, vhd.code)
		return
	}
	_, err := rw.Write([]byte(vhd.metrics.SValue))
	if err != nil {
		respErr := NewRespError("rw error", err)
		api.Error(rw, respErr, http.StatusInternalServerError)
		return
	}
}

// Accepts request with JSON body containing metrics.
//
// Responds with JSON body containing specified value.
func (api *API) ValueRootHandler(rw http.ResponseWriter, req *http.Request) {
	if strings.Contains(req.Header.Get("Content-Type"), "application/json") {
		var buf bytes.Buffer
		_, err := buf.ReadFrom(req.Body)
		if err != nil {
			respErr := NewRespError("working with request body failed", err)
			api.Error(rw, respErr, http.StatusInternalServerError)
			return
		}

		defer func() {
			closeErr := req.Body.Close()
			if closeErr != nil {
				respErr := NewRespError("working with request body failed", err)
				api.Error(rw, respErr, http.StatusInternalServerError)
				return
			}
		}()

		m := &mondata.Metrics{}
		err = json.Unmarshal(buf.Bytes(), m)
		if err != nil {
			respErr := NewRespError("unmarshaling failed", err)
			api.Error(rw, respErr, http.StatusBadRequest)
			return
		}

		vhd, respErr := getVhData(req.Context(), m, api.db)
		if respErr != nil {
			api.Error(rw, respErr, vhd.code)
			return
		}

		b, err := json.Marshal(vhd.metrics)
		if err != nil {
			respErr := NewRespError("marshaling failed", err)
			api.Error(rw, respErr, http.StatusInternalServerError)
			return
		}

		rw.Header().Add("Content-Type", "application/json; charset=utf-8")
		_, err = rw.Write(b)
		if err != nil {
			respErr := NewRespError("rw error", err)
			api.Error(rw, respErr, http.StatusInternalServerError)
			return
		}
	} else {
		respErr := NewRespError("unsupported content type", nil)
		api.Error(rw, respErr, http.StatusBadRequest)
	}
}

// Checks database connection
func (api *API) PingHandler(rw http.ResponseWriter, req *http.Request) {
	err := api.db.Ping(req.Context())
	if err != nil {
		respErr := NewRespError("connection to DB wasn't established", err)
		api.Error(rw, respErr, http.StatusInternalServerError)
		return
	}

	rw.WriteHeader(http.StatusOK)
}

// GRPC API
type GRPCAPI struct {
	pb.UnimplementedMetricsServer
	db     MDB
	logger ErrLogger
}

func NewGRPCAPI(db MDB, logger ErrLogger) *GRPCAPI {
	return &GRPCAPI{
		pb.UnimplementedMetricsServer{},
		db,
		logger,
	}
}

// Logs error and responds with error code and error message
func (api *GRPCAPI) Error(err *RespError, c codes.Code) {
	api.logger.Errorln(err)
	status.Error(c, err.Msg())
}

// Accepts requests with GRPC
//
// Updates specified metric and respond with ID if succeeded.
func (api *GRPCAPI) UpdateMetrics(ctx context.Context, in *pb.UpdateMetricsRequest) (*pb.UpdateMetricsResponse, error) {
	if in.Metrics.ID == "" {
		return nil, status.Error(codes.InvalidArgument, "name must contain a value")
	}

	var resp pb.UpdateMetricsResponse
	switch in.Metrics.MType {
	case mondata.GaugeType:
		err := api.db.SetGauge(ctx, in.Metrics.ID, in.Metrics.Value)
		if err != nil {
			return nil, status.Error(codes.Internal, "setting gauge value in db failed")
		}
		resp.ID = in.Metrics.ID
		return &resp, nil
	case mondata.CounterType:
		err := api.db.SetCounter(ctx, in.Metrics.ID, in.Metrics.Delta)
		if err != nil {
			return nil, status.Error(codes.Internal, "setting counter value in db failed")
		}
		resp.ID = in.Metrics.ID
		return &resp, nil
	default:
		return nil, status.Error(codes.InvalidArgument, "name must contain a value")
	}
}

// Accepts requests with GRPC that contains array of metric data.
//
// Updates specified metrics and respond with len if succeeded.
func (api *GRPCAPI) UpdateMetricsBatch(ctx context.Context, in *pb.UpdateMetricsBatchRequest) (*pb.UpdateMetricsBatchResponse, error) {
	fmt.Println("UPDATE METRICS BATCH")
	gm := make(map[string]float64)
	cm := make(map[string]int64)

	var resp pb.UpdateMetricsBatchResponse
	var updLen int64

	for _, rec := range in.Metrics {
		if rec.ID == "" {
			continue
		}

		updLen++
		if rec.MType == mondata.GaugeType {
			gm[rec.ID] = rec.Value

		} else if rec.MType == mondata.CounterType {
			if cv, ok := cm[rec.ID]; ok {
				cm[rec.ID] = cv + rec.Delta
				continue
			}
			cm[rec.ID] = rec.Delta
		}
	}

	if len(gm) == 0 && len(cm) == 0 {
		return nil, status.Error(codes.Internal, "nothging to update")
	}

	if len(gm) > 0 {
		if err := api.db.SetGaugeAll(ctx, gm); err != nil {
			return nil, status.Error(codes.Internal, "gauge batch update to db failed")
		}
	}

	if len(cm) > 0 {
		if err := api.db.SetCounterAll(ctx, cm); err != nil {
			return nil, status.Error(codes.Internal, "counter batch update to db failed")
		}
	}

	resp.Size = updLen
	return &resp, nil
}

// Accepts request with GRPC containing metrics.
//
// Responds with specified value.
func (api *GRPCAPI) GetMetrics(ctx context.Context, in *pb.GetMetricsRequest) (*pb.GetMetricsResponse, error) {
	if in.Metrics.ID == "" {
		return nil, status.Error(codes.InvalidArgument, "name must contain a value")
	}

	switch in.Metrics.MType {
	case mondata.GaugeType:
		v, ok, err := api.db.GetGauge(ctx, in.Metrics.ID)
		fmt.Println(v, ok, err)
		if err != nil {
			return nil, status.Error(codes.Internal, "getting gauge value from db failed")
		} else if ok {
			return &pb.GetMetricsResponse{
				Metrics: &pb.MetricsRec{
					ID: in.Metrics.ID, MType: in.Metrics.MType, Value: v,
				},
			}, nil
		}
		return nil, status.Error(codes.NotFound, "value doesn't exist in the storage")
	case mondata.CounterType:
		v, ok, err := api.db.GetCounter(ctx, in.Metrics.ID)
		if err != nil {
			return nil, status.Error(codes.Internal, "getting counter value from db failed")
		} else if ok {
			return &pb.GetMetricsResponse{
				Metrics: &pb.MetricsRec{
					ID: in.Metrics.ID, MType: in.Metrics.MType, Delta: v,
				},
			}, nil
		}
		return nil, status.Error(codes.NotFound, "value doesn't exist in the storage")
	}
	return nil, status.Error(codes.InvalidArgument, "name must contain a value")
}
