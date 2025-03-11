package main

import (
	"flag"
	"net/http"
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
	s := monserv.NewInstance(opts.addr)
	s.MountHandlers()
	err := http.ListenAndServe(opts.addr, s.Router)

	if err != nil {
		panic(err.Error())
	}
}
