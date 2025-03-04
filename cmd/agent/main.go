package main

import (
	"flag"
	"fmt"
	"regexp"

	collector "github.com/Allegathor/perfmon/internal/collector"
	"github.com/Allegathor/perfmon/internal/monclient"
)

type flags struct {
	addr           string
	reportInterval uint
	pollInterval   uint
}

var defOpts = flags{
	addr:           "http://localhost:8080",
	reportInterval: 10,
	pollInterval:   2,
}

var opts flags

func init() {
	opts.addr = defOpts.addr
	flag.Func("a", "address of a server to send metrics", func(flagValue string) error {
		matched, _ := regexp.MatchString(`https*://(\w+|\w+\.\w+):{1}\d+`, flagValue)
		fmt.Println(flagValue, defOpts.addr)
		if !matched {
			return nil
		}

		opts.addr = flagValue
		return nil
	})

	flag.UintVar(&opts.reportInterval, "r", defOpts.reportInterval, "interval (in seconds) of sending metrics to a server")
	flag.UintVar(&opts.pollInterval, "p", defOpts.pollInterval, "interval (in seconds) of reading metrics from a system")
}

func main() {
	flag.Parse()
	client := monclient.NewInstance(opts.addr, opts.reportInterval)
	col := collector.New(opts.pollInterval)
	ch := make(chan collector.Mtcs)
	go col.Monitor(ch)
	for mtcs := range ch {
		client.PollStats(mtcs.Gauge, mtcs.Counter)
	}
}
