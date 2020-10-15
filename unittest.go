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
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/pkg/labels"
)

// unitTestFile holds the contents of a single unit test file.
type unitTestFile struct {
	RuleFiles          []string       `yaml:"rule_files"`
	EvaluationInterval model.Duration `yaml:"evaluation_interval,omitempty"`
	//GroupEvalOrder     []string       `yaml:"group_eval_order"`
	Tests []testGroup `yaml:"tests"`
}

// testGroup is a group of input series and tests associated with it.
type testGroup struct {
	Interval        model.Duration   `yaml:"interval"`
	InputSeries     []inputSeries    `yaml:"input_series"`
	AlertRuleTests  []alertTestCase  `yaml:"alert_rule_test,omitempty"`
	PromqlExprTests []promqlTestCase `yaml:"promql_expr_test,omitempty"`
	ExternalLabels  labels.Labels    `yaml:"external_labels,omitempty"`
}

type inputSeries struct {
	Series string `yaml:"series"`
	Values string `yaml:"values"`
}

type alertTestCase struct {
	EvalTime  model.Duration `yaml:"eval_time"`
	Alertname string         `yaml:"alertname"`
	ExpAlerts []alert        `yaml:"exp_alerts"`
}

type alert struct {
	ExpLabels      map[string]string `yaml:"exp_labels"`
	ExpAnnotations map[string]string `yaml:"exp_annotations"`
}

type promqlTestCase struct {
	Expr       string         `yaml:"expr"`
	EvalTime   model.Duration `yaml:"eval_time"`
	ExpSamples []sample       `yaml:"exp_samples"`
}

type sample struct {
	Labels string  `yaml:"labels"`
	Value  float64 `yaml:"value"`
}
