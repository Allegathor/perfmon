package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Allegathor/perfmon/internal/monserv"
	"github.com/Allegathor/perfmon/internal/monserv/fw"
	"github.com/Allegathor/perfmon/internal/options"
	"github.com/Allegathor/perfmon/internal/repo"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/sync/errgroup"
)

type flags struct {
	addr          string
	dbConnStr     string
	mode          string
	path          string
	key           string
	storeInterval uint
	restore       bool
}

var srvOpts flags

var defSrvOpts = &flags{
	addr:          "localhost:8080",
	dbConnStr:     "",
	mode:          "dev",
	path:          "./backup.json",
	key:           "",
	storeInterval: 300,
	restore:       false,
}

func init() {
	flag.StringVar(&srvOpts.addr, "a", defSrvOpts.addr, "address to runing a server on")
	flag.StringVar(&srvOpts.dbConnStr, "d", defSrvOpts.dbConnStr, "URL for DB connection")
	flag.StringVar(&srvOpts.mode, "m", defSrvOpts.mode, "mode of running the server: dev or prod")
	flag.StringVar(&srvOpts.key, "k", defSrvOpts.key, "key for signing data")
	flag.StringVar(&srvOpts.path, "f", defSrvOpts.path, "path to backup file")
	flag.UintVar(&srvOpts.storeInterval, "i", defSrvOpts.storeInterval, "interval (in seconds) of writing to backup file")
	flag.BoolVar(&srvOpts.restore, "r", defSrvOpts.restore, "option to restore from backup file on startup")
}

func setEnv() {
	options.SetEnvStr(&srvOpts.addr, "ADDRESS")
	options.SetEnvStr(&srvOpts.dbConnStr, "DATABASE_DSN")
	options.SetEnvStr(&srvOpts.mode, "MODE")
	options.SetEnvStr(&srvOpts.key, "KEY")
	options.SetEnvStr(&srvOpts.path, "FILE_STORAGE_PATH")
	options.SetEnvUint(&srvOpts.storeInterval, "STORE_INTERVAL")
	options.SetEnvBool(&srvOpts.restore, "RESTORE")
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
	logger := initLogger(srvOpts.mode).Sugar()

	bkp := &fw.Backup{
		Path:        srvOpts.path,
		Interval:    srvOpts.storeInterval,
		Logger:      logger,
		RestoreFlag: srvOpts.restore,
	}

	db := repo.Init(context.Background(), srvOpts.dbConnStr, bkp, logger)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		db.Restore()
		wg.Done()
	}()
	wg.Wait()

	s := monserv.NewInstance(ctx, srvOpts.addr, db, srvOpts.key, logger)
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
