package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

type Client struct {
	ctx    context.Context
	client api.Client
}

func NewClient(ctx context.Context, promURL string, tokenFile string) (*Client, error) {
	u, err := url.Parse(promURL)
	if err != nil {
		return nil, errors.Wrap(err, "Invalid URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, errors.Errorf("Invalid URL scheme: %s", u.Scheme)
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
		return nil, err
	}

	return &Client{
		ctx:    ctx,
		client: client,
	}, nil
}

type bearerTokenRoundTripper struct {
	tokenFile string
	next      http.RoundTripper
}

func (b *bearerTokenRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	token, err := ioutil.ReadFile(b.tokenFile)
	if err != nil {
		return nil, err
	}

	r.Header.Add("Authorization", fmt.Sprintf("Bearer %s", strings.Trim(string(token), "\n")))

	return b.next.RoundTrip(r)
}

func (c *Client) AlertingRules() (map[string][]v1.AlertingRule, error) {
	res, err := v1.NewAPI(c.client).Rules(c.ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get rules")
	}

	alertingRules := make(map[string][]v1.AlertingRule)
	for _, group := range res.Groups {
		for _, rule := range group.Rules {
			switch v := rule.(type) {
			case v1.AlertingRule:
				alertingRules[v.Name] = append(alertingRules[v.Name], v)
			}
		}
	}

	return alertingRules, nil
}

func (c *Client) RecordingRules() (map[string][]v1.RecordingRule, error) {
	res, err := v1.NewAPI(c.client).Rules(c.ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get rules")
	}

	recordingRules := make(map[string][]v1.RecordingRule)
	for _, group := range res.Groups {
		for _, rule := range group.Rules {
			switch v := rule.(type) {
			case v1.RecordingRule:
				recordingRules[v.Name] = append(recordingRules[v.Name], v)
			}
		}
	}

	return recordingRules, nil
}
