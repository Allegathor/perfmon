package monclient

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Allegathor/perfmon/internal/collector"
	"github.com/Allegathor/perfmon/internal/mondata"
	"github.com/go-resty/resty/v2"
)

const (
	updatePath      = "/update"
	updateBatchPath = "/updates"
)

type MonClient struct {
	addr           string
	reportInterval uint
	Client         *resty.Client
}

func NewInstance(addr string, interval uint) *MonClient {
	retryDelays := []time.Duration{
		1 * time.Second,
		3 * time.Second,
		5 * time.Second,
	}

	retryCount := len(retryDelays)
	c := resty.New()
	c.
		SetRetryCount(retryCount).
		SetRetryWaitTime(0).
		SetRetryAfter(func(c *resty.Client, r *resty.Response) (time.Duration, error) {
			attempt := r.Request.Attempt

			if attempt > retryCount {
				return 0, fmt.Errorf("max retries reached")
			}

			delay := retryDelays[attempt-1]
			fmt.Printf("Retry attempt %d, waiting %v\n", attempt, delay)

			return delay, nil
		})

	m := &MonClient{
		addr:           addr,
		reportInterval: interval,
		Client:         c,
	}

	return m
}

func buildReqBody(name string, mtype string, g *float64, c *int64) []byte {
	data := &mondata.Metrics{
		ID:    name,
		MType: mtype,
	}
	if mtype == mondata.GaugeType {
		data.Value = g
	} else {
		data.Delta = c
	}
	j, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}
	return j
}

func buildReqBatchBody(gm map[string]float64, cm map[string]int64) []byte {
	mbatch := make([]mondata.Metrics, 0)

	if len(gm) == 0 && len(cm) == 0 {
		return make([]byte, 0)
	}

	for k, v := range gm {
		mbatch = append(mbatch, mondata.Metrics{
			ID:    k,
			MType: "gauge",
			Value: &v,
		})
	}

	for k, d := range cm {
		mbatch = append(mbatch, mondata.Metrics{
			ID:    k,
			MType: "counter",
			Delta: &d,
		})
	}

	j, err := json.Marshal(mbatch)
	if err != nil {
		panic(err)
	}
	return j
}

func (m *MonClient) Post(p []byte, path string) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err := zw.Write(p)
	if err != nil {
		panic(err)
	}
	zw.Close()

	resp, err := m.Client.R().
		SetHeader("Accept-Encoding", "gzip").
		SetHeader("Content-Encoding", "gzip").
		SetHeader("Content-Type", "application/json; charset=utf-8").
		SetBody(&buf).
		Post(m.addr + path)

	if err != nil {
		panic(err)
	}

	resp.RawBody().Close()
}

func (m *MonClient) PollStats(cl *collector.Collector) {
	for {
		time.Sleep(time.Duration(m.reportInterval) * time.Second)
		go func() {
			var data map[string]float64
			cl.Repo.Gauge.Read(func(tx *collector.MtcsTx[float64]) error {
				data = tx.GetAll()

				return nil
			})
			for k, v := range data {
				b := buildReqBody(k, mondata.GaugeType, &v, nil)
				m.Post(b, updatePath)
			}
		}()

		go func() {
			var data map[string]int64
			cl.Repo.Counter.Read(func(tx *collector.MtcsTx[int64]) error {
				data = tx.GetAll()

				return nil
			})
			for k, v := range data {
				b := buildReqBody(k, mondata.CounterType, nil, &v)
				m.Post(b, updatePath)
			}
		}()
	}

}

func (m *MonClient) PollStatsBatch(cl *collector.Collector) {
	for {
		time.Sleep(time.Duration(m.reportInterval) * time.Second)
		go func() {
			var (
				gm map[string]float64
				cm map[string]int64
			)

			cl.Repo.Gauge.Read(func(tx *collector.MtcsTx[float64]) error {
				gm = tx.GetAll()

				return nil
			})

			cl.Repo.Counter.Read(func(tx *collector.MtcsTx[int64]) error {
				cm = tx.GetAll()

				return nil
			})

			b := buildReqBatchBody(gm, cm)
			if len(b) > 0 {
				m.Post(b, updateBatchPath)
			}
		}()
	}
}
