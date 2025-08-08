package main

import (
	"crypto/rsa"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"runtime"
	"strings"

	"github.com/Allegathor/perfmon/internal/ciphers"
	collector "github.com/Allegathor/perfmon/internal/collector"
	"github.com/Allegathor/perfmon/internal/monclient"
	"github.com/Allegathor/perfmon/internal/options"
)

var (
	buildVersion = "N/A"
	buildDate    = "N/A"
	buildCommit  = "N/A"
)

type flags struct {
	addr           string
	key            string
	publicKeyPath  string
	rateLimit      uint
	reportInterval uint
	pollInterval   uint
}

var defOpts = &flags{
	addr:           "http://localhost:8080",
	key:            "",
	publicKeyPath:  "",
	rateLimit:      3,
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

var agOpts = &flags{
	addr: defOpts.addr,
}

func init() {
	flag.Func("a", "address of a server to send metrics", func(flagValue string) error {
		fmt.Println(flagValue, defOpts.addr)
		agOpts.addr = setAddr(flagValue, defOpts.addr)
		return nil
	})
	flag.StringVar(&agOpts.key, "k", defOpts.key, "key for signing data in requests")
	flag.StringVar(&agOpts.publicKeyPath, "crypto-key", defOpts.publicKeyPath, "path to .pem file with a public key")
	flag.UintVar(&agOpts.rateLimit, "l", defOpts.rateLimit, "maximum requests with report to a server")
	flag.UintVar(&agOpts.reportInterval, "r", defOpts.reportInterval, "interval (in seconds) of sending metrics to a server")
	flag.UintVar(&agOpts.pollInterval, "p", defOpts.pollInterval, "interval (in seconds) of reading metrics from a system")
}

func setEnv() {
	if v, ok := os.LookupEnv("ADDRESS"); ok {
		addr := setAddr(v, "")
		if addr != "" {
			agOpts.addr = addr
		}
	}
	options.SetEnvStr(&agOpts.key, "KEY")

	options.SetEnvUint(&agOpts.reportInterval, "REPORT_INTERVAL")
	options.SetEnvUint(&agOpts.pollInterval, "POLL_INTERVAL")
}

func main() {
	flag.Parse()
	setEnv()
	fmt.Printf("\nBuild version: %s\nBuild date: %s\nBuild commit: %s\n", buildVersion, buildDate, buildCommit)

	var cryptoKey *rsa.PublicKey
	if agOpts.publicKeyPath != "" {
		k, err := ciphers.ReadPublicKey(agOpts.publicKeyPath)
		if err != nil {
			log.Fatal(err)
		}
		cryptoKey = k
	}

	client := monclient.NewInstance(agOpts.addr, agOpts.key, cryptoKey, agOpts.reportInterval)
	cl := collector.New(agOpts.pollInterval)

	go cl.Monitor()
	go client.PollStatsBatch(cl, agOpts.rateLimit, 9)

	runtime.Goexit()
}
