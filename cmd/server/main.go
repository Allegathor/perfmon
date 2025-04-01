package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	monserv "github.com/Allegathor/perfmon/internal/monserv"
	"github.com/Allegathor/perfmon/internal/monserv/fw"
	"github.com/Allegathor/perfmon/internal/repo"
	"github.com/Allegathor/perfmon/internal/repo/transaction"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/sync/errgroup"
)

type flags struct {
	addr          string
	mode          string
	path          string
	storeInterval uint
	restore       bool
}

var opts flags

var defOpts = &flags{
	addr:          "localhost:8080",
	mode:          "dev",
	path:          "./backup.json",
	storeInterval: 300,
	restore:       false,
}

func init() {
	opts.addr = os.Getenv("ADDRESS")
	if opts.addr == "" {
		flag.StringVar(&opts.addr, "a", defOpts.addr, "address to runing a server on")
	}

	opts.path = os.Getenv("FILE_STORAGE_PATH")
	if opts.path == "" {
		flag.StringVar(&opts.path, "f", defOpts.path, "path to backup file")
	}

	envIntrv := os.Getenv("STORE_INTERVAL")
	if envIntrv != "" {
		i, err := strconv.ParseInt(envIntrv, 10, 32)
		if err != nil {
			fmt.Println(err.Error())
			flag.UintVar(&opts.storeInterval, "i", defOpts.storeInterval, "interval (in seconds) of writing to backup file")
		}
		opts.storeInterval = uint(i)
	} else {
		flag.UintVar(&opts.storeInterval, "i", defOpts.storeInterval, "interval (in seconds) of writing to backup file")
	}

	r, hasEnv := os.LookupEnv("RESTORE")
	if hasEnv {
		rb, err := strconv.ParseBool(r)
		if err != nil {
			fmt.Println(err.Error())
			flag.BoolVar(&opts.restore, "r", defOpts.restore, "set to restore values of repo from file at start")
		}
		opts.restore = rb
	} else {
		flag.BoolVar(&opts.restore, "r", defOpts.restore, "set to restore values of repo from file at start")
	}

	if opts.mode == "" {
		flag.StringVar(&opts.mode, "m", defOpts.mode, "set dev or prod mode")
	}
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
	logger := initLogger(opts.mode).Sugar()

	var gaugeRepo transaction.GaugeRepo
	var counterRepo transaction.CounterRepo
	gaugeRepo = repo.NewMRepo[float64]()
	counterRepo = repo.NewMRepo[int64]()

	bkp := &fw.Backup{
		Path:     opts.path,
		Interval: opts.storeInterval,
		TxGRepo:  gaugeRepo,
		TxCRepo:  counterRepo,
		Logger:   logger,
	}

	if opts.restore {
		bkp.RestorePrev()
	}

	logger.Infof("interval is %d", opts.storeInterval)

	s := monserv.NewInstance(opts.addr, logger, gaugeRepo, counterRepo)
	s.MountHandlers()

	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return s.ListenAndServe()
	})
	g.Go(func() error {
		<-gCtx.Done()
		bkp.Write()
		return s.Shutdown(context.Background())
	})

	logger.Infow("starting server", "addr:", s.Addr)
	if err = g.Wait(); err != nil {
		logger.Errorf("exit reason: %s", err)
	}
}
