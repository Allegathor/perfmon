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
}

var opts flags

var defOpts = flags{
	addr: "localhost:8080",
}

func init() {
	opts.addr = os.Getenv("ADDRESS")
	if opts.addr == "" {
		flag.StringVar(&opts.addr, "a", defOpts.addr, "address to runing a server on")
	}
}

func initLogger( /*f *os.File*/ ) *zap.Logger {
	std := zapcore.AddSync(os.Stdout)

	devcfg := zap.NewDevelopmentEncoderConfig()
	devcfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
	// prodcfg := zap.NewProductionEncoderConfig()
	// fileEncoder := zapcore.NewJSONEncoder(prodcfg)

	consoleEncoder := zapcore.NewConsoleEncoder(devcfg)
	core := zapcore.NewTee(
		zapcore.NewCore(consoleEncoder, std, zapcore.InfoLevel),
		// zapcore.NewCore(fileEncoder, zapcore.AddSync(f), zapcore.InfoLevel),
	)

	l := zap.New(core)
	defer l.Sync()

	return l
}

func main() {
	flag.Parse()

	var err error
	// f, err := os.OpenFile("logs/server.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	// if err != nil {
	// 	Info(err)
	// }
	logger := initLogger().Sugar()

	s := monserv.NewInstance(opts.addr, logger)
	s.MountHandlers()
	logger.Infow("starting server", "addr:", opts.addr)
	err = http.ListenAndServe(opts.addr, s.Router)

	if err != nil {
		panic(err.Error())
	}
}
