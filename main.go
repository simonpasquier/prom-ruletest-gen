// Copyright 2020 Simon Pasquier
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/api"
	"github.com/prometheus/common/model"
	"gopkg.in/yaml.v2"
)

var (
	help        bool
	promURL     string
	caFile      string
	tokenFile   string
	insecureTLS bool

	recordingRules = rules{}
)

type rules map[string]struct{}

// Set implements the flagset.Value interface.
func (r rules) Set(value string) error {
	for _, rule := range strings.Split(value, ",") {
		r[rule] = struct{}{}
	}
	return nil
}

// String implements the flagset.Value interface.
func (r rules) String() string {
	return strings.Join(r.asSlice(), ",")
}

func (r rules) asSlice() []string {
	rules := make([]string, 0, len(r))
	for k := range r {
		rules = append(rules, k)
	}
	return rules
}

func registerFlags() {
	flag.BoolVar(&help, "help", false, "Help message")
	flag.StringVar(&promURL, "url", "", "Prometheus base URL")
	flag.StringVar(&caFile, "ca", "", "Path to the Prometheus CA")
	flag.StringVar(&tokenFile, "token", "", "Path to the bearer token used for authentication")
	flag.BoolVar(&insecureTLS, "insecure", false, "Don't check certificate validity")
	flag.Var(&recordingRules, "recording-rule", "Recording rule for which to generate test data (can be repeated). If empty all recording rules are selected.")
}

func main() {
	registerFlags()
	flag.Parse()

	if help {
		fmt.Fprintln(os.Stderr, "Generator for Prometheus rule tests")
		flag.PrintDefaults()
		os.Exit(0)
	}

	if err := run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func run(ctx context.Context) error {
	if promURL == "" {
		return errors.New("Missing -url parameter")
	}
	u, err := url.Parse(promURL)
	if err != nil {
		return errors.Wrap(err, "Invalid URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.Errorf("Invalid URL scheme: %s", u.Scheme)
	}

	client, err := api.NewClient(api.Config{Address: promURL})
	if err != nil {
		return err
	}

	generator, err := NewGenerator(ctx, client)
	if err != nil {
		return err
	}

	keep := func(string) bool { return true }
	if len(recordingRules) > 0 {
		keep = func(name string) bool {
			_, found := recordingRules[name]
			return found
		}
	}

	testGroups, err := generator.ProcessRecordingRules(keep)
	if err != nil {
		return err
	}

	testFile := &unitTestFile{
		EvaluationInterval: model.Duration(time.Minute),
		RuleFiles:          []string{},
		Tests:              testGroups,
	}
	b, err := yaml.Marshal(testFile)
	if err != nil {
		return err
	}

	fmt.Println(string(b))

	return nil
}
