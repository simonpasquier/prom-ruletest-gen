package main

import (
	"fmt"
	"sort"

	"github.com/prometheus/prometheus/promql/parser"
)

type Inspector struct {
	client *Client
}

func NewInspector(client *Client) *Inspector {
	return &Inspector{
		client: client,
	}
}

type statistics struct {
	AlertingRules  []RuleInfo `json:"alertingRules"`
	RecordingRules []RuleInfo `json:"recordingRrules"`
}

type Query struct {
	Expression string            `json:"expression"`
	Labels     map[string]string `json:"labels"`
}

type RuleInfo struct {
	Name    string  `json:"name"`
	Queries []Query `json:"queries"`
	Metrics struct {
		Direct   []string `json:"direct"`
		Indirect []string `json:"indirect"`
	} `json:"metrics"`
}

type queryVisitor struct {
	q string

	metrics map[string]struct{}
}

// Metrics returns the list of metric names used by the rule expression.
func (qv *queryVisitor) Metrics() ([]string, error) {
	expr, err := parser.ParseExpr(qv.q)
	if err != nil {
		return nil, err
	}

	qv.metrics = make(map[string]struct{})
	if err := parser.Walk(qv, expr, nil); err != nil {
		return nil, err
	}

	var ret []string
	for k := range qv.metrics {
		ret = append(ret, k)
	}
	return ret, nil
}

// Visit implements the parser.Visitor interface.
func (qv queryVisitor) Visit(node parser.Node, _ []parser.Node) (parser.Visitor, error) {
	switch n := node.(type) {
	case *parser.VectorSelector:
		if n.Name != "" {
			// TODO: handle vector selector without metric name?
			qv.metrics[n.Name] = struct{}{}
		}
	}
	return qv, nil
}

func (i *Inspector) RecordingRules() ([]RuleInfo, error) {
	rules, err := i.client.RecordingRules()
	if err != nil {
		return nil, err
	}

	ruleNames := make([]string, 0, len(rules))
	for r := range rules {
		ruleNames = append(ruleNames, r)
	}
	sort.Strings(ruleNames)

	rulesInfo := make([]RuleInfo, 0, len(rules))
	for _, name := range ruleNames {
		ri := RuleInfo{Name: name}

		for _, r := range rules[name] {
			lbls := make(map[string]string)
			for k, v := range r.Labels {
				lbls[string(k)] = string(v)
			}
			ri.Queries = append(ri.Queries, Query{
				Expression: r.Query,
				Labels:     lbls,
			})
			qv := &queryVisitor{
				q: r.Query,
			}

			metrics, err := qv.Metrics()
			if err != nil {
				return nil, err
			}

			for _, metric := range metrics {
				if _, found := rules[metric]; found {
					ri.Metrics.Indirect = append(ri.Metrics.Indirect, metric)
					continue
				}

				ri.Metrics.Direct = append(ri.Metrics.Direct, metric)
			}
		}

		rulesInfo = append(rulesInfo, ri)
	}

	for _, ri := range rulesInfo {
		fmt.Printf("%s:\n", ri.Name)
		fmt.Println("  queries:")
		for i := range ri.Queries {
			fmt.Printf("    expr: %v\n", ri.Queries[i].Expression)
			fmt.Printf("    labels: %v\n", ri.Queries[i].Labels)
		}
		fmt.Println("  metrics:")
		fmt.Printf("    direct: %v\n", ri.Metrics.Direct)
		fmt.Printf("    indirect: %v\n", ri.Metrics.Indirect)
	}

	return rulesInfo, nil
}
