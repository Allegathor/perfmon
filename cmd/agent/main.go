package main

import (
	"flag"
	"regexp"
	"strings"

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
		addr := flagValue
		hasProto := strings.Contains(flagValue, "http://") || strings.Contains(flagValue, "https://")
		if !hasProto {
			addr = "http://" + addr
		}

		matched, _ := regexp.MatchString(`.+(\w+|\w+\.\w+):{1}\d+`, addr)
		if !matched {
			return nil
		}

		opts.addr = addr
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
