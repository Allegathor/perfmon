package main

import (
	"flag"
	"os"

	monserv "github.com/Allegathor/perfmon/internal/monserv"
)

type flags struct {
	addr string
}

var opts flags

var defOpts = flags{
	addr: "localhost:8080",
}

func init() {
	opts.addr = os.Getenv("ADDRESS")
	if opts.addr == "" {
		flag.StringVar(&opts.addr, "a", defOpts.addr, "address to runing a server on")
	}
}

func main() {
	flag.Parse()
	ms := monserv.NewInstance(opts.addr)
	err := ms.Run()

	if err != nil {
		panic(err.Error())
	}
}
