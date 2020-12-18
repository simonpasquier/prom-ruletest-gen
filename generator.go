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
	"fmt"
	"sort"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/promql/parser"
)

type Generator struct {
	ctx    context.Context
	client api.Client

	alertingRules  map[string][]v1.AlertingRule
	recordingRules map[string][]v1.RecordingRule
}

func NewGenerator(ctx context.Context, client api.Client) (*Generator, error) {
	g := &Generator{
		ctx:            ctx,
		client:         client,
		alertingRules:  make(map[string][]v1.AlertingRule),
		recordingRules: make(map[string][]v1.RecordingRule),
	}

	res, err := v1.NewAPI(g.client).Rules(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get rules")
	}

	for _, group := range res.Groups {
		for _, rule := range group.Rules {
			switch v := rule.(type) {
			case v1.AlertingRule:
				g.alertingRules[v.Name] = append(g.alertingRules[v.Name], v)
			case v1.RecordingRule:
				g.recordingRules[v.Name] = append(g.recordingRules[v.Name], v)
			}
		}
	}

	return g, nil
}

func (g *Generator) ProcessRecordingRules(keep func(string) bool) ([]testGroup, error) {
	var (
		testGroups []testGroup
		seriesSet  = map[string][]float64{}
		end        = time.Now()
		start      = end.Add(-5 * time.Minute)
	)

	for name, rules := range g.recordingRules {
		if !keep(name) {
			continue
		}

		for _, rule := range rules {
			queries, err := g.getQueries(rule.Query)
			if err != nil {
				return nil, err
			}

			for _, q := range queries {
				res, _, err := v1.NewAPI(g.client).QueryRange(g.ctx, q, v1.Range{Start: start, End: end, Step: time.Minute})
				if err != nil {
					return nil, err
				}
				if res.Type() != model.ValMatrix {
					// TODO: return error?
					continue
				}
				matrix := res.(model.Matrix)

				for _, sampleStream := range matrix {
					if _, exists := seriesSet[sampleStream.Metric.String()]; exists {
						continue
					}

					for _, sample := range sampleStream.Values {
						seriesSet[sampleStream.Metric.String()] = append(seriesSet[sampleStream.Metric.String()], float64(sample.Value))
					}
				}
			}
		}

		// Inject input series.
		input := make([]inputSeries, 0, len(seriesSet))
		orderedSeriesSet := make([]string, 0, len(seriesSet))
		for k := range seriesSet {
			orderedSeriesSet = append(orderedSeriesSet, k)
		}
		sort.Strings(orderedSeriesSet)

		for _, series := range orderedSeriesSet {
			inputSerie := inputSeries{
				Series: series,
			}
			for _, v := range seriesSet[series] {
				inputSerie.Values += fmt.Sprintf("%v ", v)
			}
			input = append(input, inputSerie)
		}

		// Inject expected PromQL test case.
		promqlTest := promqlTestCase{
			Expr:       name,
			EvalTime:   model.Duration(5 * time.Minute),
			ExpSamples: nil,
		}
		// TODO: reverse the order of queries, we need first to retrieve
		// (value, timestamp) for the last recorded value (e.g. `name[5m]`
		// because `name` alone would have a timestamp of eval time).
		// Then we should query samples for the input timeseries at the given
		// timestamp, again using a range vector to get the exact timestamps.
		res, _, err := v1.NewAPI(g.client).Query(g.ctx, name, end)
		if err != nil {
			return nil, err
		}
		vec := res.(model.Vector)
		for _, s := range vec {
			promqlTest.ExpSamples = append(
				promqlTest.ExpSamples,
				sample{
					Labels: s.Metric.String(),
					Value:  float64(s.Value),
				},
			)
		}

		testGroups = append(
			testGroups,
			testGroup{
				Interval:        model.Duration(time.Minute),
				InputSeries:     input,
				PromqlExprTests: []promqlTestCase{promqlTest},
			},
		)
	}

	return testGroups, nil
}

type visitor struct {
	keep    func(string) bool
	queries map[string][]string
}

func (v visitor) Visit(node parser.Node, _ []parser.Node) (parser.Visitor, error) {
	switch n := node.(type) {
	case *parser.VectorSelector:
		if n.Name == "" {
			// TODO: handle vector selector without metric name.
			return v, nil
		}
		if !v.keep(n.Name) {
			return v, nil
		}
		v.queries[n.Name] = append(v.queries[n.Name], n.String())
		// TODO: check for range selector.
	default:
	}
	return v, nil
}

func (g *Generator) getQueries(rule string) ([]string, error) {
	expr, err := parser.ParseExpr(rule)
	if err != nil {
		return nil, err
	}

	v := visitor{
		queries: make(map[string][]string),
		keep: func(name string) bool {
			// Skip metrics that are already generated from recording rules.
			_, found := g.recordingRules[name]
			return !found
		},
	}
	if err := parser.Walk(&v, expr, nil); err != nil {
		return nil, err
	}

	var ret []string
	for k := range v.queries {
		ret = append(ret, k)
	}
	return ret, nil
}
