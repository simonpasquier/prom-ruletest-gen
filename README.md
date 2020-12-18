`prom-ruletest-gen` is a tool to generate Prometheus rules unit tests from a running Prometheus server.

It works basically like this:

* Retrieve selected recording rules from Prometheus.
* Parse the PromQL expressions and extract series that are used by the recording.
* Retrieve samples from Prometheus for the selected recording rule(s) and associated timeseries.
* Output a unit test based on the data pulled from Prometheus.

```
$ ./prom-ruletest-gen -h
Usage of ./prom-ruletest-gen:
  -ca string
        Path to the Prometheus CA
  -help
        Help message
  -insecure
        Don't check certificate validity
  -recording-rule value
        Recording rule for which to generate test data (can be repeated). If empty all recording rules are selected.
  -token string
        Path to the bearer token used for authentication
  -url string
        Prometheus base URL
```

## License

Apache License 2.0, see [LICENSE](https://github.com/simonpasquier/prom-ruletest-gen/blob/master/LICENSE).
