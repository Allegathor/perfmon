package fw

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"time"

	"github.com/Allegathor/perfmon/internal/repo"
	"github.com/Allegathor/perfmon/internal/repo/transaction"
	"go.uber.org/zap"
)

type Backup struct {
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

	var data, pt1, pt2 []byte
	if len(gVals) > 0 {
		pt1, err = json.Marshal(gVals)
		if err != nil {
			return err
		}
		data = append([]byte{}, '[')
	}

	if len(cVals) > 0 {
		pt2, err = json.Marshal(cVals)
		if err != nil {
			return err
		}
	}

	if len(pt1) > 2 || len(pt2) > 2 {
		data = append(data, pt1...)
		data = append(data, ',')
		data = append(data, pt2...)
		data = append(data, ']')
	}

	_, err = f.Write(data)
	if err != nil {
		return err
	}

	return f.Close()
}

func (b *Backup) Run() {
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
