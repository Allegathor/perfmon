package fw

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	mu          sync.Mutex
	Path        string
	Interval    uint
	Logger      *zap.SugaredLogger
	RestoreFlag bool
}

func (b *Backup) ShouldRestore() bool {
	return b.RestoreFlag
}

func (b *Backup) RestorePrev(db repo.MetricsRepo) error {
	if !b.ShouldRestore() {
		return nil
	}

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

	var (
		gParseErr error = nil
		gEmptyErr error = nil
	)

	if len(gj) > 2 {
		gParseErr = json.Unmarshal(gj, &gaugeData)
		if gParseErr == nil {
			db.SetGaugeAll(context.TODO(), gaugeData)
		}
	} else {
		gEmptyErr = fmt.Errorf("couldn't read any meaningful gauge values from the file: %s;", b.Path)
	}

	if len(cj) > 2 {
		err = json.Unmarshal(cj, &counterData)
		if err != nil && gParseErr != nil {
			return errors.Join(gParseErr, err)
		}
		db.SetCounterAll(context.TODO(), counterData)
	} else if gEmptyErr != nil {
		return errors.Join(
			gEmptyErr,
			fmt.Errorf("couldn't read any meaningful counter values from the file: %s;", b.Path),
		)
	}

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
	} else {
		pt1 = []byte{'{', '}'}
	}

	if len(cVals) > 0 {
		pt2, err = json.Marshal(cVals)
		if err != nil {
			return err
		}
	}

	if len(pt1) < 3 && len(pt2) < 3 {
		return fmt.Errorf("nothing to write to the backup file: %s", b.Path)
	}

	var data []byte
	var slb [][]byte
	data = append(data, '[')

	slb = append(slb, pt1)

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

	ticker := time.NewTicker(time.Duration(300) * time.Second)
	for {
		select {
		case <-ticker.C:
			// 	wg.Add(1)
			// 	go func() {
			// 		err := b.Write(db, false)
			// 		defer wg.Done()
			// 		if err != nil {
			// 			b.Logger.Errorf("scheduled backup failed with err: %v", err)
			// 			return
			// 		}
			// b.Logger.Info("scheduled backup success")
		// 	}()
		case <-ctx.Done():
			ticker.Stop()
			wg.Wait()
			err := b.Write(db, true)
			if err != nil {
				b.Logger.Errorf("shutdown backup failed with error: %v", err)
				return err
			}
			b.Logger.Info("shutdown backup success")
			return nil
		}
	}
}
