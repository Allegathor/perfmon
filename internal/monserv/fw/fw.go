package fw

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/Allegathor/perfmon/internal/mondata"
	"github.com/Allegathor/perfmon/internal/repo"
	"go.uber.org/zap"
)

type Getters interface {
	GetGaugeAll(ctx context.Context) (mondata.GaugeMap, error)
	GetCounterAll(ctx context.Context) (mondata.CounterMap, error)
}

type Setters interface {
	SetGaugeAll(ctx context.Context, gaugeMap mondata.GaugeMap) error
	SetCounterAll(ctx context.Context, gaugeMap mondata.CounterMap) error
}

type MDB interface {
	Getters
	Setters
}

type Backup struct {
	Path        string
	Interval    uint
	RestoreFlag bool
	Logger      *zap.SugaredLogger
	mu          sync.Mutex
}

func (b *Backup) ShouldRestore() bool {
	return b.RestoreFlag
}

func (b *Backup) RestorePrev(db repo.MetricsRepo) error {
	b.mu.Lock()
	defer b.mu.Unlock()
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
	if s.Scan() {
		gj = bytes.Trim(s.Bytes(), "[,]")
	}

	if s.Scan() {
		cj = bytes.Trim(s.Bytes(), "[,]")
	}

	var gaugeData mondata.GaugeMap
	var counterData mondata.CounterMap

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

	db.SetGaugeAll(context.TODO(), gaugeData)
	db.SetCounterAll(context.TODO(), counterData)

	b.Logger.Info("restoring from backup success")
	return nil
}

func (b *Backup) Write(db repo.MetricsRepo, truncateFlag bool) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if truncateFlag {
		os.Truncate(b.Path, 0)
	}

	f, err := os.OpenFile(b.Path, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	gVals, _ := db.GetGaugeAll(context.TODO())
	cVals, _ := db.GetCounterAll(context.TODO())

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

func (b *Backup) Schedule(ctx context.Context, db repo.MetricsRepo) error {
	var wg sync.WaitGroup

	for {
		ticker := time.NewTicker(time.Duration(300) * time.Second)
		defer ticker.Stop()
		select {
		case <-ticker.C:
			// wg.Add(1)
			// go func() {
			// 	err := b.Write(db, false)
			// 	defer wg.Done()
			// 	if err != nil {
			// 		b.Logger.Errorf("scheduled backup failed with err: %v", err)

			// 		return
			// 	}
			// 	b.Logger.Info("scheduled backup success")
			// }()
		case <-ctx.Done():
			wg.Wait()
			err := b.Write(db, true)
			if err != nil {
				b.Logger.Errorf("shutdown backup failed with err: %v", err)
			}
			b.Logger.Info("shutdown backup success")
			return nil
		}
	}
}
