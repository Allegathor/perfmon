package main

import (
	"crypto/rsa"
	"encoding/json"
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

const (
	configPath = "agent_config.json"
)

var (
	buildVersion = "N/A"
	buildDate    = "N/A"
	buildCommit  = "N/A"
)

type flags struct {
	Addr           string `json:"address"`
	Key            string `json:"key"`
	PublicKeyPath  string `json:"crypto_key"`
	RateLimit      uint   `json:"rate_limit"`
	ReportInterval uint   `json:"report_interval"`
	PollInterval   uint   `json:"poll_interval"`
}

var defOpts = &flags{
	Addr:           "http://localhost:8080",
	Key:            "",
	PublicKeyPath:  "",
	RateLimit:      3,
	ReportInterval: 10,
	PollInterval:   2,
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
	Addr: defOpts.Addr,
}

func init() {
	info, err := os.Stat(configPath)
	if os.IsNotExist(err) {
		fmt.Println("config file not found")
	} else if !info.IsDir() {
		fmt.Printf("found config file:\n%v\n", info)
		f, err := os.ReadFile(configPath)
		if err != nil {
			fmt.Println("failed to read config file")
		}

		jsonErr := json.Unmarshal(f, defOpts)
		if jsonErr != nil {
			fmt.Println("failed to parse json from config file")
		}
	}
	options.SetEnvStr(&agOpts.key, "KEY")

	flag.Func("a", "address of a server to send metrics", func(flagValue string) error {
		fmt.Println(flagValue, defOpts.Addr)
		agOpts.Addr = setAddr(flagValue, defOpts.Addr)
		return nil
	})
	flag.StringVar(&agOpts.Key, "k", defOpts.Key, "key for signing data in requests")
	flag.StringVar(&agOpts.PublicKeyPath, "crypto-key", defOpts.PublicKeyPath, "path to .pem file with a public key")
	flag.UintVar(&agOpts.RateLimit, "l", defOpts.RateLimit, "maximum requests with report to a server")
	flag.UintVar(&agOpts.ReportInterval, "r", defOpts.ReportInterval, "interval (in seconds) of sending metrics to a server")
	flag.UintVar(&agOpts.PollInterval, "p", defOpts.PollInterval, "interval (in seconds) of reading metrics from a system")
}

func setEnv() {
	if v, ok := os.LookupEnv("ADDRESS"); ok {
		addr := setAddr(v, "")
		if addr != "" {
			agOpts.Addr = addr
		}
	}
	options.SetEnvStr(&agOpts.Key, "KEY")

	options.SetEnvUint(&agOpts.ReportInterval, "REPORT_INTERVAL")
	options.SetEnvUint(&agOpts.PollInterval, "POLL_INTERVAL")
}

func main() {
	flag.Parse()
	setEnv()
	fmt.Printf("\nBuild version: %s\nBuild date: %s\nBuild commit: %s\n", buildVersion, buildDate, buildCommit)

	var cryptoKey *rsa.PublicKey
	if agOpts.PublicKeyPath != "" {
		k, err := ciphers.ReadPublicKey(agOpts.PublicKeyPath)
		if err != nil {
			log.Fatal(err)
		}
		cryptoKey = k
	}

	client := monclient.NewInstance(agOpts.Addr, agOpts.Key, cryptoKey, agOpts.ReportInterval)
	cl := collector.New(agOpts.PollInterval)

	go cl.Monitor()
	go client.PollStatsBatch(cl, agOpts.RateLimit, 9)

	runtime.Goexit()
}
