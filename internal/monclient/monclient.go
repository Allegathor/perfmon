package monclient

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"time"

	"github.com/Allegathor/perfmon/internal/collector"
	"github.com/Allegathor/perfmon/internal/mondata"
	"github.com/go-resty/resty/v2"
)

type MonClient struct {
	addr           string
	updatePath     string
	reportInterval uint
	Client         *resty.Client
}

func NewInstance(addr string, interval uint) *MonClient {
	c := resty.New()
	c.
		SetRetryCount(2).
		SetRetryWaitTime(time.Duration(interval/2) * time.Second)

	m := &MonClient{
		addr:           addr,
		updatePath:     "/update",
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

func (m *MonClient) Post(p []byte) {
	var buf bytes.Buffer
	zb := gzip.NewWriter(&buf)
	_, err := zb.Write(p)
	if err != nil {
		panic(err)
	}
	zb.Close()

	resp, err := m.Client.R().
		SetHeader("Accept-Encoding", "gzip").
		SetHeader("Content-Encoding", "gzip").
		SetHeader("Content-Type", "application/json; charset=utf-8").
		SetBody(&buf).
		Post(m.addr + m.updatePath)

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
				m.Post(b)
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
				m.Post(b)
			}
		}()
	}

}
