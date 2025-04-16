package transaction

import "github.com/Allegathor/perfmon/internal/mondata"

type Tx[T mondata.VTypes] interface {
	Lock()
	Unlock()
}

type TxQry[T mondata.VTypes] interface {
	Tx[T]
	Get(name string) (T, bool)
	GetAll() map[string]T
}

type TxExec[T mondata.VTypes] interface {
	Tx[T]
	Set(name string, v T)
	SetAccum(name string, v T)
	SetAll(map[string]T)
}

type GaugeRepo interface {
	Read(func(TxQry[float64]) error) error
	Update(func(TxExec[float64]) error) error
}

type CounterRepo interface {
	Read(func(TxQry[int64]) error) error
	Update(func(TxExec[int64]) error) error
}
