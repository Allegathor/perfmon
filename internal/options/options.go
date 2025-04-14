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

func (opts *Options) SetStr(envName string, flagName string, value *string, defaultValue string, usage string) {
	*value = os.Getenv(envName)
	if *value == "" {
		opts.fgs.StringVar(value, flagName, defaultValue, usage)
	}
}

func (opts *Options) SetInt(envName string, flagName string, value *uint, defaultValue uint, usage string) {
	if ev := os.Getenv(envName); ev != "" {
		i, err := strconv.Atoi(ev)
		if err != nil {
			fmt.Println(err.Error())
			opts.fgs.UintVar(value, flagName, defaultValue, usage)
			return
		}
		*value = uint(i)
	} else {
		opts.fgs.UintVar(value, flagName, defaultValue, usage)
	}
}

func (opts *Options) SetBool(envName string, flagName string, value *bool, defaultValue bool, usage string) {
	if ev, hasEv := os.LookupEnv(envName); hasEv {
		evb, err := strconv.ParseBool(ev)
		if err != nil {
			fmt.Println(err.Error())
			opts.fgs.BoolVar(value, flagName, defaultValue, usage)
			return
		}
		*value = evb
	} else {
		opts.fgs.BoolVar(value, flagName, defaultValue, usage)
	}
}

func (opts *Options) Parse() {
	opts.fgs.Parse(os.Args[1:])
}
