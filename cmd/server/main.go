package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	monserv "github.com/Allegathor/perfmon/internal/monserv"
	"github.com/Allegathor/perfmon/internal/monserv/fw"
	"github.com/Allegathor/perfmon/internal/opts"
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
	storeInterval uint
	restore       bool
}

var srvOpts flags

var defSrvOpts = &flags{
	addr:          "localhost:8080",
	dbConnStr:     "",
	mode:          "dev",
	path:          "./backup.json",
	storeInterval: 300,
	restore:       false,
}

func init() {
	opts.SetStr("ADDRESS", "a", &srvOpts.addr, defSrvOpts.addr, "address to runing a server on")
	opts.SetStr("DATABASE_DSN", "d", &srvOpts.dbConnStr, defSrvOpts.dbConnStr, "URL for DB connection")
	opts.SetStr("MODE", "m", &srvOpts.mode, defSrvOpts.mode, "mode of running the server: dev or prod")
	opts.SetStr("FILE_STORAGE_PATH", "f", &srvOpts.path, defSrvOpts.path, "path to backup file")
	opts.SetInt("STORE_INTERVAL", "s", &srvOpts.storeInterval, defSrvOpts.storeInterval, "interval (in seconds) of writing to backup file")
	opts.SetBool("RESTORE", "r", &srvOpts.restore, defSrvOpts.restore, "whether to restore from backup file on startup")
}

func initLogger(mode string) *zap.Logger {
	var core zapcore.Core
	if mode == "prod" {
		f, err := os.OpenFile("logs/server.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
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
		RestoreFlag: srvOpts.restore,
		Logger:      logger,
	}

	db := repo.Init(context.Background(), srvOpts.dbConnStr, bkp)

	s := monserv.NewInstance(srvOpts.addr, db, logger)
	s.MountHandlers()

	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return s.ListenAndServe()
	})
	g.Go(func() error {
		<-gCtx.Done()
		db.Dump()
		return s.Shutdown(context.Background())
	})

	logger.Infow("starting server", "addr:", s.Addr)
	if err = g.Wait(); err != nil {
		logger.Errorf("exit reason: %s", err)
	}
}
