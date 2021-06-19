# prometheus-slo

Generates Prometheus rules for alerting on SLOs. Based on https://developers.soundcloud.com/blog/alerting-on-slos.

## Usage

### Build and Run

```
$ go build -o prometheus-slo *.go
$ ./prometheus-slo # Takes prometheus-slo.yaml as the default config file
2021/06/19 18:18:01 Generating cortex.yaml
2021/06/19 18:18:01 Generating example.yaml
$ ./prometheus-slo --config=<config_file_path> # For config at other paths
```

### Config file format

```yaml
slo_files:
  <file-path>:
    - service: <string> # Service name
      slo_name: <string> # SLO name
      alertname: <string> # Name of the SLO alert
      alert_summary: <string> # Summary of this alert
      # PromQL query giving the count of requests *within SLO*.
      # Put $__range for the range in the range selector.
      success_query: <string>
      # PromQL query giving the total count of requests.
      # Put $__range for the range in the range selector.
      total_query: <string> 
      threshold: <string> # Quantile values in terms of %, example: 99.9, 99.95

```

Look at [`prometheus-slo.yaml`](./prometheus-slo.yaml) for an example of the config.
