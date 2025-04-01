package fw

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/Allegathor/perfmon/internal/repo"
	"github.com/Allegathor/perfmon/internal/repo/transaction"
	"go.uber.org/zap"
)

type Backup struct {
	Ctx      context.Context
	Path     string
	Interval uint
	TxGRepo  transaction.GaugeRepo
	TxCRepo  transaction.CounterRepo
	Logger   *zap.SugaredLogger
}

func (b *Backup) RestorePrev() error {
	f, err := os.OpenFile(b.Path, os.O_CREATE|os.O_RDONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	s.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		for i, b := range data {
			if b == ',' && data[i+1] == '{' {
				return i + 1, data[:i+1], nil
			}
		}
		if atEOF && len(data) > 0 {
			return len(data), data, nil
		}
		return 0, nil, nil
	})

	var gj, cj []byte
	s.Scan()
	gj = bytes.Trim(s.Bytes(), "[,]")
	s.Scan()
	cj = bytes.Trim(s.Bytes(), "[,]")

	var gaugeData repo.GaugeMap
	var counterData repo.CounterMap

	if len(gj) < 3 {
		return nil
	}
	err = json.Unmarshal(gj, &gaugeData)
	if err != nil {
		return err
	}

	if len(cj) < 3 {
		return nil
	}
	err = json.Unmarshal(cj, &counterData)
	if err != nil {
		return err
	}

	go b.TxGRepo.Update(func(tx transaction.Tx[float64]) error {
		tx.SetAll(gaugeData)

		return nil
	})

	go b.TxCRepo.Update(func(tx transaction.Tx[int64]) error {
		tx.SetAll(counterData)

		return nil
	})

	return nil
}

func (b *Backup) Write() error {
	f, err := os.OpenFile(b.Path, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	gch := make(chan map[string]float64)
	cch := make(chan map[string]int64)

	go b.TxGRepo.Read(func(tx transaction.Tx[float64]) error {
		gch <- tx.GetAll()
		return nil
	})

	go b.TxCRepo.Read(func(tx transaction.Tx[int64]) error {
		cch <- tx.GetAll()
		tx.GetAll()

		return nil
	})

	gVals := <-gch
	cVals := <-cch

	var pt1, pt2 []byte
	if len(gVals) > 0 {
		pt1, err = json.Marshal(gVals)
		if err != nil {
			return err
		}
	}

	if len(cVals) > 0 {
		pt2, err = json.Marshal(cVals)
		if err != nil {
			return err
		}
	}

	if len(pt1) < 3 && len(pt2) < 3 {
		return nil
	}

	var data []byte
	var slb [][]byte
	data = append(data, '[')

	if !(len(pt1) < 3) {
		slb = append(slb, pt1)
	}

	if !(len(pt2) < 3) {
		slb = append(slb, pt2)
	}

	data = append(data, bytes.Join(slb, []byte(","))...)
	data = append(data, ']')

	_, err = f.Write(data)
	if err != nil {
		return err
	}

	return nil
}

func (b *Backup) Run() error {
	for {
		time.Sleep(time.Duration(b.Interval) * time.Second)
		go func() {
			err := b.Write()
			if err != nil {
				b.Logger.Errorw("error in Run() goroutine, errMsg:", err.Error())
			}
		}()
	}
}
