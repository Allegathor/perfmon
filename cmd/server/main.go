package main

import (
	monserv "github.com/Allegathor/perfmon/internal/monserv"
)

func main() {
	ms := monserv.NewInstance(8080)
	err := ms.Run()

	if err != nil {
		panic(err.Error())
	}
}
