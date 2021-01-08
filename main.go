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
	promURL     string
	caFile      string
	tokenFile   string
	insecureTLS bool

	recordingRules = rules{}
	alertingRules  = rules{}

	commands = map[string]func(ctx context.Context) error{
		"generate": generate,
		"inspect":  inspect,
	}
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

func registerFlags(f *flag.FlagSet) {
	f.StringVar(&promURL, "url", "", "Prometheus base URL")
	f.StringVar(&caFile, "ca", "", "Path to the Prometheus CA")
	f.StringVar(&tokenFile, "token-file", "", "Path to the bearer token used for authentication")
	f.BoolVar(&insecureTLS, "insecure", false, "Don't check certificate validity")
	f.Var(&recordingRules, "recording-rule", "Recording rule to select (can be repeated). If empty all recording rules are selected.")
	f.Var(&alertingRules, "alerting-rule", "Alerting rule to select (can be repeated). If empty all alerting rules are selected.")
}

func main() {
	fset := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	fset.Usage = func() {
		fmt.Fprintln(os.Stderr, "Prometheus rule inspector and test generator")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "  [FLAGS] (generate|inspect)")
		fmt.Fprintln(os.Stderr)
		fset.PrintDefaults()
	}
	registerFlags(fset)

	err := fset.Parse(os.Args[1:])
	if err == flag.ErrHelp {
		os.Exit(0)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "error parsing command-line arguments:", err)
		os.Exit(2)
	}

	if promURL == "" {
		fmt.Fprintln(os.Stderr, "missing '-url' argument")
		os.Exit(2)
	}

	if len(fset.Args()) > 1 {
		fmt.Fprintln(os.Stderr, "can only pass one command")
		os.Exit(2)
	}

	cmd := fset.Arg(0)
	if cmd == "" {
		cmd = "inspect"
	}

	fn, found := commands[cmd]
	if !found {
		fmt.Fprintln(os.Stderr, "invalid command:", cmd)
		os.Exit(2)
	}

	if err := fn(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func inspect(ctx context.Context) error {
	client, err := NewClient(ctx, promURL, tokenFile)
	if err != nil {
		return err
	}
	inspector := NewInspector(client)

	// List alerting/recording rules.
	// List metrics used by alerting/recording rules (+ distinguish between scraped and recorded metrics).
	// For each metric, list cardinality information.
	inspector.RecordingRules()

	return nil
}

func generate(ctx context.Context) error {
	u, err := url.Parse(promURL)
	if err != nil {
		return errors.Wrap(err, "Invalid URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.Errorf("Invalid URL scheme: %s", u.Scheme)
	}

	rt := api.DefaultRoundTripper
	if tokenFile != "" {
		rt = &bearerTokenRoundTripper{
			tokenFile: tokenFile,
			next:      rt,
		}
	}

	client, err := api.NewClient(api.Config{Address: promURL, RoundTripper: rt})
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
