package monclient

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"time"

	"github.com/Allegathor/perfmon/internal/mondata"
)

type MonClient struct {
	addr         string
	updatePath   string
	pollInterval uint
	*http.Client
}

func NewInstance(addr string, interval uint) *MonClient {
	m := &MonClient{
		addr:         addr,
		updatePath:   "/update",
		pollInterval: interval,
		Client:       &http.Client{},
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

	req, err := http.NewRequest(http.MethodPost, m.addr+m.updatePath, &buf)
	if err != nil {
		panic(err)
	}
	req.Header.Add("Content-Type", "application/json")

	req.Header.Add("Accept-Encoding", "gzip")
	req.Header.Add("Content-Encoding", "gzip")
	req.Header.Add("Content-Type", "application/json; charset=utf-8")

	resp, err := m.Do(req)
	if err != nil {
		panic(err)
	}

	resp.Body.Close()
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
