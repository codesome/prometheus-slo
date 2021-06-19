package main

import (
	"flag"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"strings"

	"gopkg.in/yaml.v2"
)

func main() {
	var cfgFile string
	flag.StringVar(&cfgFile, "config", "prometheus-slo.yaml", "File path containing the config to generate SLO rules.")
	flag.Parse()

	b, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		log.Fatalf("Failed to read config file, err=%s\n", err.Error())
	}

	cfg := struct {
		SloFiles map[string][]alertInput `yaml:"slo_files"`
	}{
		SloFiles: make(map[string][]alertInput),
	}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		log.Fatalf("Failed to unmarshal config file, err=%s\n", err.Error())
	}

	for fn, sloAlertInput := range cfg.SloFiles {
		log.Printf("Generating %s\n", fn)
		groups := makeRules(sloAlertInput)
		out, err := yaml.Marshal(map[string][]alertGroup{
			"groups": groups,
		})
		if err != nil {
			log.Fatalf("Failed to marshal slo rules file, err=%s\n", err.Error())
		}

		err = ioutil.WriteFile(fn, out, fs.ModePerm)
		if err != nil {
			log.Fatalf("Failed to write slo rules file, err=%s\n", err.Error())
		}
	}
}

func makeRules(ais []alertInput) (g []alertGroup) {
	for _, ai := range ais {
		rr, ar := makeRule(ai)
		ag := alertGroup{
			Name: fmt.Sprintf("%s_%s_slo", ai.Service, ai.SloName),
		}
		for i := range rr {
			ag.Rules = append(ag.Rules, &(rr[i]))
		}
		for i := range ar {
			ag.Rules = append(ag.Rules, &(ar[i]))
		}
		g = append(g, ag)
	}

	return g
}

func makeRule(ai alertInput) (rr []recordingRule, ar []alertRule) {
	for _, w := range shortSloRecordWindows {
		// Rule for the numerator: rate of successful requests.
		rr = append(rr, recordingRule{
			Record: fmt.Sprintf("cluster_namespace:%s_%s_successful_requests_total:rate%s", ai.Service, ai.SloName, w),
			Expr:   strings.ReplaceAll(ai.SuccessQuery, "$__range", w),
		})
		// Rule for the denominator: rate of total requests.
		rr = append(rr, recordingRule{
			Record: fmt.Sprintf("cluster_namespace:%s_%s_requests_total:rate%s", ai.Service, ai.SloName, w),
			Expr:   strings.ReplaceAll(ai.TotalQuery, "$__range", w),
		})
	}

	// We derive the longer ranges from the shorter ones using avg_over_time.
	for _, w := range longSloRecordWindows {
		rr = append(rr, recordingRule{
			Record: fmt.Sprintf("cluster_namespace:%s_%s_successful_requests_total:rate%s", ai.Service, ai.SloName, w),
			Expr:   fmt.Sprintf("avg_over_time(cluster_namespace:%s_%s_successful_requests_total:rate1h[%s])", ai.Service, ai.SloName, w),
		})
		rr = append(rr, recordingRule{
			Record: fmt.Sprintf("cluster_namespace:%s_%s_requests_total:rate%s", ai.Service, ai.SloName, w),
			Expr:   fmt.Sprintf("avg_over_time(cluster_namespace:%s_%s_requests_total:rate1h[%s])", ai.Service, ai.SloName, w),
		})
		rr = append(rr, recordingRule{
			Record: fmt.Sprintf("cluster_namespace:%s_%s_success_per_request:rate%s", ai.Service, ai.SloName, w),
			Expr:   fmt.Sprintf("avg_over_time(cluster_namespace:%s_%s_success_per_request:rate1h[%s])", ai.Service, ai.SloName, w),
		})
	}

	// Some rules for the ratios.
	for _, w := range append(shortSloRecordWindows, longSloRecordWindows...) {
		rr = append(rr, recordingRule{
			Record: fmt.Sprintf("cluster_namespace:%s_%s_success_per_request:ratio_rate%s", ai.Service, ai.SloName, w),
			Expr: fmt.Sprintf(`(cluster_namespace:%s_%s_successful_requests_total:rate%s / cluster_namespace:%s_%s_requests_total:rate%s)`,
				ai.Service, ai.SloName, w, ai.Service, ai.SloName, w),
		})
	}

	// Alerts.
	for _, w := range sloAlertWindows {
		ar = append(ar, alertRule{
			Alert: ai.AlertName,
			Expr: fmt.Sprintf(`(((1 - cluster_namespace:%s_%s_success_per_request:ratio_rate%s) * 100 > (100 - %s) * %.1f) and ((1 - cluster_namespace:%s_%s_success_per_request:ratio_rate%s) * 100 > (100 - %s) * %.1f))`,
				ai.Service, ai.SloName, w.longPeriod, ai.Threshold, w.factor,
				ai.Service, ai.SloName, w.shortPeriod, ai.Threshold, w.factor),
			For: w.forPeriod,
			Labels: map[string]string{
				"severity": w.severity,
				"period":   w.longPeriod,
			},
			Annotations: map[string]string{
				"summary":     ai.AlertSummary,
				"description": fmt.Sprintf("{{ $value | printf `%%.2f` }}%% of {{ $labels.job }}'s requests in the last %s are failing or too slow to meet the SLO.", w.longPeriod),
			},
		})
	}

	return rr, ar
}

var shortSloRecordWindows = []string{"5m", "30m", "1h"}

var longSloRecordWindows = []string{"2h", "6h", "1d", "3d"}

var sloAlertWindows = []sloAlertWindow{
	{longPeriod: "1h", shortPeriod: "5m", forPeriod: "2m", factor: 14.4, severity: "critical"},
	{longPeriod: "6h", shortPeriod: "30m", forPeriod: "15m", factor: 6, severity: "critical"},
	{longPeriod: "1d", shortPeriod: "2h", forPeriod: "1h", factor: 3, severity: "warning"},
	{longPeriod: "3d", shortPeriod: "6h", forPeriod: "3h", factor: 1, severity: "warning"},
}

type alertInput struct {
	Service      string `yaml:"service"`
	SloName      string `yaml:"slo_name"`
	AlertName    string `yaml:"alertname"`
	AlertSummary string `yaml:"alert_summary"`
	SuccessQuery string `yaml:"success_query"`
	TotalQuery   string `yaml:"total_query"`
	Threshold    string `yaml:"threshold"`
}

type alertGroup struct {
	Name  string        `yaml:"name"`
	Rules []interface{} `yaml:"rules"`
}

type recordingRule struct {
	Record string `yaml:"record"`
	Expr   string `yaml:"expr"`
}

type alertRule struct {
	Alert       string            `yaml:"alert"`
	Expr        string            `yaml:"expr"`
	For         string            `yaml:"for"`
	Labels      map[string]string `yaml:"labels"`
	Annotations map[string]string `yaml:"annotations"`
}

type sloAlertWindow struct {
	longPeriod  string
	shortPeriod string
	forPeriod   string
	factor      float32
	severity    string
}
