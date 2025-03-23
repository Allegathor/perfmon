package main

import (
	"flag"
	"net/http"
	"os"

	monserv "github.com/Allegathor/perfmon/internal/monserv"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type flags struct {
	addr string
	mode string
}

var opts flags

var defOpts = flags{
	addr: "localhost:8080",
	mode: "dev",
}

func init() {
	opts.addr = os.Getenv("ADDRESS")
	if opts.addr == "" {
		flag.StringVar(&opts.addr, "a", defOpts.addr, "address to runing a server on")
	}
	if opts.mode == "" {
		flag.StringVar(&opts.mode, "m", defOpts.mode, "set dev or production mode")
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

	var err error
	logger := initLogger(opts.mode).Sugar()

	s := monserv.NewInstance(opts.addr, logger)
	s.MountHandlers()
	logger.Infow("starting server", "addr:", opts.addr)
	err = http.ListenAndServe(opts.addr, s.Router)

	if err != nil {
		panic(err.Error())
	}
}
