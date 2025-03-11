package main

import (
	"flag"
	"os"
	"regexp"
	"strconv"
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

func setAddr(value string, defaultValue string) string {
	hasProto := strings.Contains(value, "http://") || strings.Contains(value, "https://")
	if !hasProto {
		value = "http://" + value
	}

	matched, _ := regexp.MatchString(`.+(\w+|\w+\.\w+):{1}\d+`, value)
	if !matched {
		return defaultValue
	}

	return value
}

var opts flags

func init() {
	opts.addr = defOpts.addr
	envAddr := os.Getenv("ADDRESS")

	if envAddr != "" {
		opts.addr = setAddr(envAddr, defOpts.addr)
	} else {
		flag.Func("a", "address of a server to send metrics", func(flagValue string) error {
			opts.addr = setAddr(flagValue, defOpts.addr)
			return nil
		})
	}

	envr := os.Getenv("REPORT_INTERVAL")
	if envr != "" {
		r, err := strconv.ParseUint(envr, 10, 32)
		if err != nil {
			opts.reportInterval = uint(r)
		}
	} else {
		flag.UintVar(&opts.reportInterval, "r", defOpts.reportInterval, "interval (in seconds) of sending metrics to a server")
	}

	envp := os.Getenv("POLL_INTERVAL")
	if envp != "" {
		p, err := strconv.ParseUint(envp, 10, 32)
		if err != nil {
			opts.reportInterval = uint(p)
		}
	} else {
		flag.UintVar(&opts.pollInterval, "p", defOpts.pollInterval, "interval (in seconds) of reading metrics from a system")
	}
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
