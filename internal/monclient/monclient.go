package monclient

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"log"
	"sync"
	"time"

	"github.com/Allegathor/perfmon/internal/ciphers"
	"github.com/Allegathor/perfmon/internal/collector"
	"github.com/Allegathor/perfmon/internal/mondata"
	pb "github.com/Allegathor/perfmon/internal/proto"
	"github.com/go-resty/resty/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	updatePath      = "/update"
	updateBatchPath = "/updates"
	ipAddress       = "172.18.254.1"
)

type MonClient struct {
	addr           string
	reportInterval uint
	h              hash.Hash
	cryptoKey      *rsa.PublicKey
	Client         *resty.Client
}

func NewInstance(addr string, key string, cryptoKey *rsa.PublicKey, interval uint) *MonClient {
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
		cryptoKey:      cryptoKey,
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

func buildBatch(gm map[string]float64, cm map[string]int64) []mondata.Metrics {
	mbatch := make([]mondata.Metrics, 0)

	if len(gm) == 0 && len(cm) == 0 {
		return mbatch
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

	return mbatch
}

func buildBatchPb(gm map[string]float64, cm map[string]int64) []*pb.MetricsRec {
	mbatch := make([]*pb.MetricsRec, 0)

	if len(gm) == 0 && len(cm) == 0 {
		return mbatch
	}

	for k, v := range gm {
		mbatch = append(mbatch, &pb.MetricsRec{
			ID:    k,
			MType: "gauge",
			Value: v,
		})
	}

	for k, d := range cm {
		mbatch = append(mbatch, &pb.MetricsRec{
			ID:    k,
			MType: "counter",
			Delta: d,
		})
	}

	return mbatch
}

func buildReqBatchBody(gm map[string]float64, cm map[string]int64) []byte {
	mbatch := buildBatch(gm, cm)

	j, err := json.Marshal(mbatch)
	if err != nil {
		log.Fatal(err)
	}
	return j
}

func (m *MonClient) Post(p []byte, path string) {
	var buf bytes.Buffer
	if m.cryptoKey != nil {
		encMsg, err := ciphers.EncryptMsg(m.cryptoKey, p)
		if err != nil {
			log.Fatal(err)
		}
		p = encMsg
	}

	zw := gzip.NewWriter(&buf)
	_, err := zw.Write(p)
	if err != nil {
		log.Fatal(err)
	}
	zw.Close()

	req := m.Client.R()

	if m.h != nil {
		_, wErr := m.h.Write(p)
		if wErr != nil {
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
		SetHeader("X-Real-IP", ipAddress).
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

func (m *MonClient) PollWorker(idx uint, reps <-chan *Report, wg *sync.WaitGroup) {
	defer wg.Done()
	for r := range reps {
		b := buildReqBatchBody(r.gm, r.cm)
		if len(b) > 0 {
			m.Post(b, updateBatchPath)
		}
		fmt.Printf("worker %d complete job N%d\n", idx, r.id)
	}
}

func readStats(id int64, cl *collector.Collector, wg *sync.WaitGroup, repsCh chan<- *Report) {
	var (
		gm map[string]float64
		cm map[string]int64
	)
	defer wg.Done()

	cl.Repo.Gauge.Read(func(tx *collector.MtcsTx[float64]) error {
		gm = tx.GetAll()

		return nil
	})

	cl.Repo.Counter.Read(func(tx *collector.MtcsTx[int64]) error {
		cm = tx.GetAll()

		return nil
	})

	repsCh <- &Report{gm, cm, id}
}

func (m *MonClient) PollStatsBatch(ctx context.Context, cl *collector.Collector, wpoolCount uint, chCap uint) error {
	var id int64
	repsCh := make(chan *Report, chCap)

	var poolWG sync.WaitGroup
	for i := range wpoolCount {
		poolWG.Add(1)
		go m.PollWorker(i, repsCh, &poolWG)
	}

	var tickerWG sync.WaitGroup
	ticker := time.NewTicker(time.Duration(m.reportInterval) * time.Second)
	for {
		select {
		case <-ticker.C:
			id++
			tickerWG.Add(1)
			go readStats(id, cl, &tickerWG, repsCh)
		case <-ctx.Done():
			ticker.Stop()
			tickerWG.Wait()
			close(repsCh)
			poolWG.Wait()

			return errors.New("client graceful shutdown")
		}
	}
}

type MonClientGRPC struct {
	addr           string
	reportInterval uint
	h              hash.Hash
	cryptoKey      *rsa.PublicKey
}

func NewInstanceGRPC(addr string, key string, cryptoKey *rsa.PublicKey, interval uint) *MonClientGRPC {
	var h hash.Hash = nil
	if key != "" {
		h = hmac.New(sha256.New, []byte(key))
	}

	m := &MonClientGRPC{
		addr:           addr,
		h:              h,
		cryptoKey:      cryptoKey,
		reportInterval: interval,
	}

	return m
}

func (m *MonClientGRPC) PollWorker(ctx context.Context, pbClient pb.MetricsClient, idx uint, reps <-chan *Report, wg *sync.WaitGroup) {
	defer wg.Done()
	for r := range reps {
		b := buildBatchPb(r.gm, r.cm)
		if len(b) > 0 {
			fmt.Println("UPDATE SUCCESS")
			pbClient.UpdateMetricsBatch(ctx, &pb.UpdateMetricsBatchRequest{
				Metrics: b,
			})
		}
		fmt.Printf("worker %d complete job N%d\n", idx, r.id)
	}
}

func (m *MonClientGRPC) PollStatsBatch(ctx context.Context, cl *collector.Collector, wpoolCount uint, chCap uint) error {
	conn, err := grpc.Dial(m.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()
	pbClient := pb.NewMetricsClient(conn)

	var id int64
	repsCh := make(chan *Report, chCap)

	var poolWG sync.WaitGroup
	for i := range wpoolCount {
		poolWG.Add(1)
		go m.PollWorker(ctx, pbClient, i, repsCh, &poolWG)
	}

	var tickerWG sync.WaitGroup
	ticker := time.NewTicker(time.Duration(m.reportInterval) * time.Second)
	for {
		select {
		case <-ticker.C:
			id++
			tickerWG.Add(1)
			go readStats(id, cl, &tickerWG, repsCh)
		case <-ctx.Done():
			ticker.Stop()
			tickerWG.Wait()
			close(repsCh)
			poolWG.Wait()

			return errors.New("client graceful shutdown")
		}
	}
}
