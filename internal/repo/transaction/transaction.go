package transaction

import "github.com/Allegathor/perfmon/internal/mondata"

type Tx[T mondata.VTypes] interface {
	Get(name string) (T, bool)
	GetAll() map[string]T
	Set(name string, v T)
	SetAccum(name string, v T)
	SetAll(map[string]T)
	Lock()
	Unlock()
}

type GaugeRepo interface {
	Read(func(Tx[float64]) error) error
	Update(func(Tx[float64]) error) error
}

type CounterRepo interface {
	Read(func(Tx[int64]) error) error
	Update(func(Tx[int64]) error) error
}
