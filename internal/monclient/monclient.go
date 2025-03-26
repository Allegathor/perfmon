package monclient

import (
	"bytes"
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

func buildBuf(name string, mtype string, g *float64, c *int64) *bytes.Buffer {
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
	return bytes.NewBuffer(j)
}

func (m *MonClient) Post(buf *bytes.Buffer) {
	req, err := http.NewRequest(http.MethodPost, m.addr+m.updatePath, buf)
	if err != nil {
		panic(err)
	}
	req.Header.Add("Content-Type", "application/json")

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
				b := buildBuf(k, mondata.GaugeType, &v, nil)
				m.Post(b)
			}
		}()

		go func() {
			for k, v := range cm {
				b := buildBuf(k, mondata.CounterType, nil, &v)
				m.Post(b)
			}

		}()
	}

}
