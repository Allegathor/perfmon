package options

import (
	"fmt"
	"os"
	"strconv"
)

func SetEnvStr(p *string, envName string) {
	v := os.Getenv(envName)
	if v != "" {
		*p = v
	}
}

func SetEnvUint(p *uint, envName string) {
	if v := os.Getenv(envName); v != "" {
		i, err := strconv.Atoi(v)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		*p = uint(i)
	}
}

func SetEnvBool(p *bool, envName string) {
	if v, ok := os.LookupEnv(envName); ok {
		b, err := strconv.ParseBool(v)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		*p = b
	}
}
