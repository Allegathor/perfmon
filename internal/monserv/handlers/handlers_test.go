package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Allegathor/perfmon/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// func marshalWithErr(v any) ([]byte, error) {
// 	return nil, errors.New("Unable to marshal the value")
// }

func TestCreateUpdateHandler(t *testing.T) {
	type want struct {
		contentType string
		code        int
		success     bool
		errMsg      string
	}
	tests := []struct {
		name    string
		request string
		storage storage.MetricsStorage
		want    want
	}{
		{
			name:    "positive test #1",
			request: "/update/counter/PollCount/1",
			storage: storage.MetricsStorage{},
			want: want{
				contentType: "",
				code:        200,
				success:     true,
				errMsg:      "",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := CreateUpdateHandler(&tt.storage)
			req := httptest.NewRequest(http.MethodPost, tt.request, nil)
			w := httptest.NewRecorder()
			http.HandleFunc("/update/{type}/{name}/{value}", func(w http.ResponseWriter, r *http.Request) { h(w, req) })

			res := w.Result()
			assert.Equal(t, tt.want.code, res.StatusCode)
			assert.Equal(t, tt.want.contentType, res.Header.Get("Content-Type"))

			_, err := io.ReadAll(res.Body)
			require.NoError(t, err)

			err = res.Body.Close()
			require.NoError(t, err)

			// if tt.want.success {
			// }

			// assert.Equal(t, tt.want.errMsg, string(body))

		})
	}
}
