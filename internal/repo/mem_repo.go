package repo

import (
	"sync"

	"github.com/Allegathor/perfmon/internal/mondata"
	"github.com/Allegathor/perfmon/internal/repo/transaction"
)

type (
	GaugeMap   = map[string]mondata.GaugeVType
	CounterMap = map[string]mondata.CounterVType
)

type MRepo[T mondata.VTypes] struct {
	mu   sync.RWMutex
	Data map[string]T
}

type MRepoTx[T mondata.VTypes] struct {
	repo     *MRepo[T]
	writable bool
}

func (tx *MRepoTx[T]) Get(name string) (T, bool) {
	v, ok := tx.repo.Data[name]
	return v, ok
}

func (tx *MRepoTx[T]) GetAll() map[string]T {
	return tx.repo.Data
}

func (tx *MRepoTx[T]) Set(name string, v T) {
	tx.repo.Data[name] = v
}

func (tx *MRepoTx[T]) SetAccum(name string, v T) {
	if _, ok := tx.repo.Data[name]; ok {
		tx.repo.Data[name] += v
		return
	}
	tx.repo.Data[name] = v
}

func (tx *MRepoTx[T]) SetAll(data map[string]T) {
	tx.repo.Data = data
}

func (tx *MRepoTx[T]) Lock() {
	if tx.writable {
		tx.repo.mu.Lock()
	} else {
		tx.repo.mu.RLock()
	}
}

func (tx *MRepoTx[T]) Unlock() {
	if tx.writable {
		tx.repo.mu.Unlock()
	} else {
		tx.repo.mu.RUnlock()
	}
}

func NewMRepo[T mondata.VTypes]() *MRepo[T] {
	return &MRepo[T]{
		Data: make(map[string]T),
	}
}

func (r *MRepo[T]) Begin(writable bool) (transaction.Tx[T], error) {
	tx := &MRepoTx[T]{
		repo:     r,
		writable: writable,
	}
	tx.Lock()

	return tx, nil
}

func (r *MRepo[T]) managed(writable bool, fn func(transaction.Tx[T]) error) (err error) {
	tx, err := r.Begin(writable)
	if err != nil {
		return err
	}

	defer func() {
		tx.Unlock()
	}()

	err = fn(tx)
	return nil
}

func (r *MRepo[T]) Read(fn func(transaction.Tx[T]) error) error {
	return r.managed(false, fn)
}

func (r *MRepo[T]) Update(fn func(transaction.Tx[T]) error) error {
	return r.managed(true, fn)
}
