# prometheus-slo

Generates Prometheus rules for alerting on SLOs. Based on https://developers.soundcloud.com/blog/alerting-on-slos.

## Usage

Look at [`prometheus-slo.yaml`](./prometheus-slo.yaml) for the example config.

```
$ go build -o prometheus-slo *.go
$ ./prometheus-slo # Takes prometheus-slo.yaml as the default file
2021/06/19 18:18:01 Generating cortex.yaml
2021/06/19 18:18:01 Generating example.yaml
$ ./prometheus-slo --config=<config_file_path> # For config at other paths
```
