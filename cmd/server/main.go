package main

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Allegathor/perfmon/internal/ciphers"
	"github.com/Allegathor/perfmon/internal/monserv"
	"github.com/Allegathor/perfmon/internal/monserv/fw"
	"github.com/Allegathor/perfmon/internal/options"
	"github.com/Allegathor/perfmon/internal/repo"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/sync/errgroup"
)

const (
	configPath = "server_config.json"
)

var (
	buildVersion = "N/A"
	buildDate    = "N/A"
	buildCommit  = "N/A"
)

type flags struct {
	Addr           string `json:"address"`
	DBConnStr      string `json:"database_dsn"`
	Mode           string `json:"mode"`
	Path           string `json:"store_file"`
	Key            string `json:"key"`
	PrivateKeyPath string `json:"crypto_key"`
	StoreInterval  uint   `json:"store_interval"`
	Restore        bool   `json:"restore"`
}

var srvOpts flags

var defSrvOpts = &flags{
	Addr:           "localhost:8080",
	DBConnStr:      "",
	Mode:           "dev",
	Path:           "./backup.json",
	PrivateKeyPath: "",
	Key:            "",
	StoreInterval:  300,
	Restore:        false,
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

		jsonErr := json.Unmarshal(f, defSrvOpts)
		if jsonErr != nil {
			fmt.Println("failed to parse json from config file")
		}
	}

	flag.StringVar(&srvOpts.Addr, "a", defSrvOpts.Addr, "address to runing a server on")
	flag.StringVar(&srvOpts.DBConnStr, "d", defSrvOpts.DBConnStr, "URL for DB connection")
	flag.StringVar(&srvOpts.Mode, "m", defSrvOpts.Mode, "mode of running the server: dev or prod")
	flag.StringVar(&srvOpts.Key, "k", defSrvOpts.Key, "key for signing data")
	flag.StringVar(&srvOpts.PrivateKeyPath, "crypto-key", defSrvOpts.PrivateKeyPath, "path to .pem file with a private key")
	flag.StringVar(&srvOpts.Path, "f", defSrvOpts.Path, "path to backup file")
	flag.UintVar(&srvOpts.StoreInterval, "i", defSrvOpts.StoreInterval, "interval (in seconds) of writing to backup file")
	flag.BoolVar(&srvOpts.Restore, "r", defSrvOpts.Restore, "option to restore from backup file on startup")
}

func setEnv() {
	options.SetEnvStr(&srvOpts.Addr, "ADDRESS")
	options.SetEnvStr(&srvOpts.DBConnStr, "DATABASE_DSN")
	options.SetEnvStr(&srvOpts.Mode, "MODE")
	options.SetEnvStr(&srvOpts.Key, "KEY")
	options.SetEnvStr(&srvOpts.PrivateKeyPath, "CRYPTO_KEY")
	options.SetEnvStr(&srvOpts.Path, "FILE_STORAGE_PATH")
	options.SetEnvUint(&srvOpts.StoreInterval, "STORE_INTERVAL")
	options.SetEnvBool(&srvOpts.Restore, "RESTORE")
}

func initLogger(mode string) *zap.Logger {
	var core zapcore.Core
	if mode == "prod" {
		f, err := os.OpenFile("server.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}

		prodcfg := zap.NewProductionEncoderConfig()
		fileEncoder := zapcore.NewJSONEncoder(prodcfg)
		sync := zapcore.AddSync(f)
		core = zapcore.NewTee(
			zapcore.NewCore(fileEncoder, sync, zapcore.InfoLevel),
		)
	} else {
		std := zapcore.AddSync(os.Stdout)

		devcfg := zap.NewDevelopmentEncoderConfig()
		devcfg.EncodeLevel = zapcore.CapitalColorLevelEncoder

		consoleEncoder := zapcore.NewConsoleEncoder(devcfg)
		core = zapcore.NewTee(
			zapcore.NewCore(consoleEncoder, std, zapcore.InfoLevel),
		)
	}

	l := zap.New(core)
	defer l.Sync()

	return l
}

func main() {
	flag.Parse()
	setEnv()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)

		<-c
		cancel()
	}()

	var err error
	logger := initLogger(srvOpts.Mode).Sugar()
	logger.Infof("\nBuild version: %s\nBuild date: %s\nBuild commit: %s\n", buildVersion, buildDate, buildCommit)

	bkp := &fw.Backup{
		Path:        srvOpts.Path,
		Interval:    srvOpts.StoreInterval,
		Logger:      logger,
		RestoreFlag: srvOpts.Restore,
	}

	db := repo.Init(context.Background(), srvOpts.DBConnStr, bkp, logger)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		db.Restore()
		wg.Done()
	}()
	wg.Wait()

	var cryptoKey *rsa.PrivateKey
	if srvOpts.PrivateKeyPath != "" {
		cryptoKey, err = ciphers.ReadPrivateKey(srvOpts.PrivateKeyPath)
		if err != nil {
			logger.Errorf("erorr reading private key from file", err)
			os.Exit(-1)
		}
	}

	s := monserv.NewInstance(ctx, srvOpts.Addr, db, srvOpts.Key, cryptoKey, logger)
	s.MountHandlers()

	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return s.ListenAndServe()
	})
	g.Go(func() error {
		return db.ScheduleBackup(gCtx)
	})
	g.Go(func() error {
		<-gCtx.Done()
		timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		db.Close()

		go func() error {
			<-timeoutCtx.Done()
			if timeoutCtx.Err() == context.DeadlineExceeded {
				return errors.New("timed out performing graceful shutdown")
			}

			return nil
		}()

		return s.Shutdown(timeoutCtx)
	})

	logger.Infow("server was started", "addr:", s.Addr)
	if err = g.Wait(); err != nil {
		logger.Errorf("exit reason: %s", err)
	}
}
