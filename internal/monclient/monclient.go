package monclient

import (
	"bytes"
	"compress/gzip"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash"
	"log"
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
	h              hash.Hash
	Client         *resty.Client
}

func NewInstance(addr string, key string, interval uint) *MonClient {
	retryCount := 3

	c := resty.New()
	c.
		SetRetryCount(retryCount).
		SetRetryWaitTime(0).
		SetRetryAfter(func(c *resty.Client, r *resty.Response) (time.Duration, error) {
			attempt := r.Request.Attempt

			if attempt > retryCount {
				return 0, fmt.Errorf("max retries reached")
			}
			delay := time.Second + time.Duration(attempt-1)*2*time.Second

			fmt.Printf("Retry attempt %d, waiting %v\n", attempt, delay)

			return delay, nil
		})

	var h hash.Hash = nil
	if key != "" {
		h = hmac.New(sha256.New, []byte(key))
	}

	m := &MonClient{
		addr:           addr,
		h:              h,
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
		log.Fatal(err)
	}
	return j
}

func (m *MonClient) Post(p []byte, path string) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err := zw.Write(p)
	if err != nil {
		log.Fatal(err)
	}
	zw.Close()

	req := m.Client.R()

	if m.h != nil {
		_, err := m.h.Write(p)
		if err != nil {
			log.Fatal(err)
		}

		signStr := base64.URLEncoding.EncodeToString(m.h.Sum(nil))
		req.SetHeader("HashSHA256", signStr)
		m.h.Reset()
	}

	resp, err := req.
		SetHeader("Accept-Encoding", "gzip").
		SetHeader("Content-Encoding", "gzip").
		SetHeader("Content-Type", "application/json; charset=utf-8").
		SetBody(&buf).
		Post(m.addr + path)

	if err != nil {
		log.Fatal(err)
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

type Report struct {
	gm map[string]float64
	cm map[string]int64
	id int64
}

func (m *MonClient) PollWorker(idx uint, reps <-chan *Report, out chan<- int64) {
	for r := range reps {
		b := buildReqBatchBody(r.gm, r.cm)
		if len(b) > 0 {
			m.Post(b, updateBatchPath)
		}
		fmt.Printf("worker %d complete job N%d\n", idx, r.id)
		out <- r.id
	}
}

func (m *MonClient) PollStatsBatch(cl *collector.Collector, wpoolCount uint, chCap uint) {
	var id int64
	repsCh := make(chan *Report, chCap)
	reqCh := make(chan int64)

	for i := range wpoolCount {
		go m.PollWorker(i, repsCh, reqCh)
	}

	ticker := time.NewTicker(time.Duration(m.reportInterval) * time.Second)
	for {
		select {
		case <-ticker.C:
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

				repsCh <- &Report{gm, cm, id}

				id++
			}()
		case <-reqCh:
			// TODO
		}
	}
}
