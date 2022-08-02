package main

import (
	tokensvm "ava-tokens/tokens-vm"
	"flag"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	versionKey = "version"
)

func buildFlagSet() *flag.FlagSet {
	fs := flag.NewFlagSet(tokensvm.Name, flag.ContinueOnError)

	fs.Bool(versionKey, false, "If true, prints Version and quit")

	return fs
}

func getViper() (*viper.Viper, error) {
	v := viper.New()

	fs := buildFlagSet()
	pflag.CommandLine.AddGoFlagSet(fs)
	pflag.Parse()
	if err := v.BindPFlags(pflag.CommandLine); err != nil {
		return nil, err
	}

	return v, nil
}

func PrintVersion() (bool, error) {
	v, err := getViper()
	if err != nil {
		return false, err
	}

	return v.GetBool(versionKey), nil
}
