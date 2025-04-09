package opts

import (
	"flag"
	"fmt"
	"os"
	"strconv"
)

func SetStr(envName string, flagName string, value *string, defaultValue string, usage string) {
	*value = os.Getenv(envName)
	if *value == "" {
		flag.StringVar(value, flagName, defaultValue, usage)
	}
}

func SetInt(envName string, flagName string, value *uint, defaultValue uint, usage string) {
	ev := os.Getenv(envName)
	if ev != "" {
		i, err := strconv.Atoi(ev)
		if err != nil {
			fmt.Println(err.Error())
			flag.UintVar(value, flagName, defaultValue, usage)
		}
		*value = uint(i)
	} else {
		flag.UintVar(value, flagName, defaultValue, usage)
	}
}

func SetBool(envName string, flagName string, value *bool, defaultValue bool, usage string) {
	ev, hasEv := os.LookupEnv(envName)
	if hasEv {
		evb, err := strconv.ParseBool(ev)
		if err != nil {
			fmt.Println(err.Error())
			flag.BoolVar(value, flagName, defaultValue, usage)
		}
		*value = evb
	} else {
		flag.BoolVar(value, flagName, defaultValue, usage)
	}
}
