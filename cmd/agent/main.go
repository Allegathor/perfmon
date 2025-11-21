package main

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Allegathor/perfmon/internal/ciphers"
	collector "github.com/Allegathor/perfmon/internal/collector"
	"github.com/Allegathor/perfmon/internal/monclient"
	"github.com/Allegathor/perfmon/internal/options"
	"golang.org/x/sync/errgroup"
)

const (
	configPath        = "agent_config.json"
	shutdownTimeout   = 10 * time.Second
	readingKeyErrCode = -1
	clientPollCap     = 9
)

var (
	buildVersion = "N/A"
	buildDate    = "N/A"
	buildCommit  = "N/A"
)

type AgentProtocols struct {
	http string
	grpc string
}

var protocols = AgentProtocols{
	http: "http",
	grpc: "grpc",
}

type flags struct {
	Addr           string `json:"address"`
	Key            string `json:"key"`
	PublicKeyPath  string `json:"crypto_key"`
	RateLimit      uint   `json:"rate_limit"`
	ReportInterval uint   `json:"report_interval"`
	PollInterval   uint   `json:"poll_interval"`
	Protocol       string `json:"protocol"`
}

var defOpts = &flags{
	Addr:           "localhost:8080",
	Key:            "",
	PublicKeyPath:  "",
	RateLimit:      3,
	ReportInterval: 10,
	PollInterval:   2,
	Protocol:       protocols.http,
}

var agOpts = &flags{
	Addr:     defOpts.Addr,
	Protocol: defOpts.Protocol,
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

	flag.StringVar(&agOpts.Addr, "a", defOpts.Addr, "address of a server to send metrics")
	flag.StringVar(&agOpts.Key, "k", defOpts.Key, "key for signing data in requests")
	flag.StringVar(&agOpts.PublicKeyPath, "crypto-key", defOpts.PublicKeyPath, "path to .pem file with a public key")
	flag.StringVar(&agOpts.Protocol, "protocol", defOpts.Protocol, "select protocol: http or grpc")
	flag.UintVar(&agOpts.RateLimit, "l", defOpts.RateLimit, "maximum requests with report to a server")
	flag.UintVar(&agOpts.ReportInterval, "r", defOpts.ReportInterval, "interval (in seconds) of sending metrics to a server")
	flag.UintVar(&agOpts.PollInterval, "p", defOpts.PollInterval, "interval (in seconds) of reading metrics from a system")
}

func setEnv() {
	if v, ok := os.LookupEnv("ADDRESS"); ok {
		if v != "" {
			agOpts.Addr = v
		}
	}

	options.SetEnvStr(&agOpts.Protocol, "PROTOCOL")
	options.SetEnvStr(&agOpts.Key, "KEY")

	options.SetEnvUint(&agOpts.ReportInterval, "REPORT_INTERVAL")
	options.SetEnvUint(&agOpts.PollInterval, "POLL_INTERVAL")
}

func main() {
	flag.Parse()
	setEnv()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)

		<-c
		cancel()
	}()

	fmt.Printf("\nBuild version: %s\nBuild date: %s\nBuild commit: %s\n", buildVersion, buildDate, buildCommit)

	var cryptoKey *rsa.PublicKey
	if agOpts.PublicKeyPath != "" {
		k, err := ciphers.ReadPublicKey(agOpts.PublicKeyPath)
		if err != nil {
			log.Fatal(err)
		}
		cryptoKey = k
	}

	switch agOpts.Protocol {
	case protocols.http:
		client := monclient.NewInstance(agOpts.Addr, agOpts.Key, cryptoKey, agOpts.ReportInterval)
		cl := collector.New(agOpts.PollInterval)

		g, gCtx := errgroup.WithContext(ctx)
		g.Go(func() error {
			return cl.Monitor(gCtx)
		})
		g.Go(func() error {
			return client.PollStatsBatch(gCtx, cl, agOpts.RateLimit, clientPollCap)
		})
		g.Go(func() error {
			<-gCtx.Done()
			timeoutCtx, cancel := context.WithTimeout(ctx, shutdownTimeout)
			defer cancel()

			go func() error {
				<-timeoutCtx.Done()
				if timeoutCtx.Err() == context.DeadlineExceeded {
					return errors.New("timed out performing graceful shutdown")
				}

				return nil
			}()

			return errors.New("agent shutdown")
		})

		fmt.Printf("agent was started, addr:%s\n", agOpts.Addr)
		if err := g.Wait(); err != nil {
			fmt.Printf("exit reason: %s\n", err)
		}
	case protocols.grpc:
		client := monclient.NewInstanceGRPC(agOpts.Addr, agOpts.Key, cryptoKey, agOpts.ReportInterval)
		cl := collector.New(agOpts.PollInterval)

		g, gCtx := errgroup.WithContext(ctx)
		g.Go(func() error {
			return cl.Monitor(gCtx)
		})
		g.Go(func() error {
			return client.PollStatsBatch(gCtx, cl, agOpts.RateLimit, clientPollCap)
		})
		g.Go(func() error {
			<-gCtx.Done()
			timeoutCtx, cancel := context.WithTimeout(ctx, shutdownTimeout)
			defer cancel()

			go func() error {
				<-timeoutCtx.Done()
				if timeoutCtx.Err() == context.DeadlineExceeded {
					return errors.New("timed out performing graceful shutdown")
				}

				return nil
			}()

			return errors.New("agent shutdown")
		})

		fmt.Printf("grpc agent was started, addr:%s\n", agOpts.Addr)
		if err := g.Wait(); err != nil {
			fmt.Printf("exit reason: %s\n", err)
		}
	}
}
