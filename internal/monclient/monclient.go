package monclient

import (
	"fmt"
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
		updatePath:   "update",
		pollInterval: interval,
		Client:       &http.Client{},
	}

	return m
}

func (m *MonClient) PostGauge(name string, v float64) {
	path := name + "/" + mondata.FormatGauge(v)
	u := fmt.Sprintf("%s/%s/%s/%s", m.addr, m.updatePath, mondata.GaugeType, path)

	req, err := http.NewRequest(http.MethodPost, u, http.NoBody)
	if err != nil {
		panic(err)
	}

	resp, err := m.Do(req)
	if err != nil {
		panic(err)
	}

	resp.Body.Close()
}

func (m *MonClient) PostCounter(name string, v int64) {
	path := name + "/" + mondata.FormatCounter(v)
	u := fmt.Sprintf("%s/%s/%s/%s", m.addr, m.updatePath, mondata.CounterType, path)

	req, err := http.NewRequest(http.MethodPost, u, http.NoBody)
	if err != nil {
		panic(err)
	}

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
				m.PostGauge(k, v)
			}
		}()

		go func() {
			for k, v := range cm {
				m.PostCounter(k, v)
			}

		}()
	}

}
