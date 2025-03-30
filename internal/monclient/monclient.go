package monclient

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"time"

	"github.com/Allegathor/perfmon/internal/mondata"
	"github.com/go-resty/resty/v2"
)

type MonClient struct {
	addr         string
	updatePath   string
	pollInterval uint
	Client       *resty.Client
}

func NewInstance(addr string, interval uint) *MonClient {
	c := resty.New()
	c.
		SetRetryCount(3).
		SetRetryWaitTime(30 * time.Second).
		SetRetryMaxWaitTime(90 * time.Second)

	m := &MonClient{
		addr:         addr,
		updatePath:   "/update",
		pollInterval: interval,
		Client:       c,
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

func (m *MonClient) PollStats(gm map[string]float64, cm map[string]int64) {

	for {
		time.Sleep(time.Duration(m.pollInterval) * time.Second)
		go func() {
			for k, v := range gm {
				b := buildReqBody(k, mondata.GaugeType, &v, nil)
				m.Post(b)
			}
		}()

		go func() {
			for k, v := range cm {
				b := buildReqBody(k, mondata.CounterType, nil, &v)
				m.Post(b)
			}

		}()
	}

}
