package handlers

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
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

type ErrLoggerMock struct{}

func (l *ErrLoggerMock) Errorln(...any) {}
