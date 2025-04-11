package safe

import (
	"sync"

	"github.com/Allegathor/perfmon/internal/mondata"
	"github.com/Allegathor/perfmon/internal/repo/transaction"
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
	for k, v := range data {
		if _, ok := tx.repo.Data[k]; ok {
			tx.repo.Data[k] = v
			continue
		}
		tx.repo.Data[k] = v
	}
}

func (tx *MRepoTx[T]) SetAccumAll(data map[string]T) {
	for k, v := range data {
		if _, ok := tx.repo.Data[k]; ok {
			tx.repo.Data[k] += v
			continue
		}
		tx.repo.Data[k] = v
	}
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

func (r *MRepo[T]) Begin(writable bool) (*MRepoTx[T], error) {
	tx := &MRepoTx[T]{
		repo:     r,
		writable: writable,
	}
	tx.Lock()

	return tx, nil
}

func (r *MRepo[T]) Read(fn func(transaction.TxQry[T]) error) error {
	tx, err := r.Begin(false)
	if err != nil {
		return err
	}

	defer func() {
		tx.Unlock()
	}()

	if err = fn(tx); err != nil {
		return err
	}
	return nil
}

func (r *MRepo[T]) Update(fn func(transaction.TxExec[T]) error) error {
	tx, err := r.Begin(true)
	if err != nil {
		return err
	}

	defer func() {
		tx.Unlock()
	}()

	if err = fn(tx); err != nil {
		return err
	}
	return nil
}
