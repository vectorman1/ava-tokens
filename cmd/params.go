/*
 * Copyright 2022 LimeChain Ltd.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
