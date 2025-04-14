package options

import (
	"flag"
	"fmt"
	"os"
	"strconv"
)

type Options struct {
	fgs *flag.FlagSet
}

func New(name string) *Options {
	return &Options{
		fgs: flag.NewFlagSet(name, flag.ExitOnError),
	}
}

func (opts *Options) SetStr(envName string, flagName string, p *string, defaultValue string, usage string) {
	var fv string
	opts.fgs.StringVar(&fv, flagName, defaultValue, usage)
	*p = os.Getenv(envName)
	if *p == "" {
		*p = fv
	}
}

func (opts *Options) SetInt(envName string, flagName string, p *uint, defaultValue uint, usage string) {
	var fv uint
	opts.fgs.UintVar(&fv, flagName, defaultValue, usage)
	if ev := os.Getenv(envName); ev != "" {
		i, err := strconv.Atoi(ev)
		if err != nil {
			fmt.Println(err.Error())
			*p = fv
			return
		}
		*p = uint(i)
	} else {
		*p = fv
	}
}

func (opts *Options) SetBool(envName string, flagName string, p *bool, defaultValue bool, usage string) {
	var fv bool
	opts.fgs.BoolVar(&fv, flagName, defaultValue, usage)
	if ev, ok := os.LookupEnv(envName); ok {
		b, err := strconv.ParseBool(ev)
		if err != nil {
			fmt.Println(err.Error())
			*p = fv
			return
		}
		*p = b
	} else {
		*p = fv
	}
}

func (opts *Options) Parse() {
	opts.fgs.Parse(os.Args[1:])
}
