package monclient

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	defcfg "github.com/Allegathor/perfmon/internal"
)

type MonClient struct {
	baseURL      string
	port         int
	updatePath   string
	pollInterval uint8
	*http.Client
}

func NewInstance(base string, p int, interval uint8) *MonClient {
	m := &MonClient{
		baseURL:      fmt.Sprintf("%s:%d", base, p),
		port:         p,
		updatePath:   "update",
		pollInterval: interval,
		Client:       &http.Client{},
	}

	return m
}

func (m *MonClient) PostGauge(name string, v float64) {
	path := name + "/" + strings.TrimRight(strconv.FormatFloat(v, 'f', -1, 64), "0.")
	u := fmt.Sprintf("%s/%s/%s/%s", m.baseURL, m.updatePath, defcfg.TypeGauge, path)

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
	path := name + "/" + strconv.FormatInt(v, 10)
	u := fmt.Sprintf("%s/%s/%s/%s", m.baseURL, m.updatePath, defcfg.TypeCounter, path)

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
