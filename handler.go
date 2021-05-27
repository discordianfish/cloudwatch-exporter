package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type handler struct {
	pathPrefix              string
	logger                  log.Logger
	errorCounter            prometheus.Counter
	durationSummary         *prometheus.SummaryVec
	reporterDurationSummary *prometheus.SummaryVec
}

func newHandler(logger log.Logger, pathPrefix string, durationSummary *prometheus.SummaryVec, errorCounter prometheus.Counter, reporterDurationSummary *prometheus.SummaryVec) *handler {
	return &handler{
		pathPrefix:              pathPrefix,
		logger:                  logger,
		errorCounter:            errorCounter,
		durationSummary:         durationSummary,
		reporterDurationSummary: reporterDurationSummary,
	}
}

// To parses URLs like
// - /metrics/AWS/EC2/DiskReadBytes
// - /metrics/AWS/EC2/
// - /metrics/Glue/glue.driver.s3.filesystem.write_bytes
// - /metrics/upshot-ingest-metrics-production/TotalLeases
// We split by / and take
// - last element as metricName
// - other elements as namespace
func (h *handler) parsePath(path string) (string, string) {
	path = path[len(h.pathPrefix):]
	var (
		parts      = strings.Split(path, "/")
		metricName = parts[len(parts)-1]
		namespace  = strings.Join(parts[:len(parts)-1], "/")
	)
	return namespace, metricName
}

func configFromQuery(query url.Values) (*reporterConfig, error) {
	config := &reporterConfig{
		delayDuration: 600 * time.Second,
		rangeDuration: 600 * time.Second,
		period:        60,
		stat:          "Maximum",
	}
	for k, v := range query {
		if len(v) == 0 {
			return nil, fmt.Errorf("query parameter %s has no values", k)
		}
		value := v[0]
		switch k {
		case "delay", "range", "period":
			n, err := strconv.Atoi(value)
			if err != nil {
				return nil, err
			}
			switch k {
			case "delay":
				config.delayDuration = time.Duration(n) * time.Second
			case "range":
				config.rangeDuration = time.Duration(n) * time.Second
			case "period":
				config.period = int32(n)
			}
		case "stat":
			config.stat = value
		}
	}
	return config, nil
}

// ServeHTTP implements http.Handler.
func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	level.Debug(h.logger).Log("msg", "got request", "path", r.URL.Path)

	namespace, metricName := h.parsePath(r.URL.Path)
	if namespace == "" {
		h.errorCounter.Inc()
		http.Error(w, "Namespace required", http.StatusBadRequest)
		return
	}
	if metricName == "" {
		h.errorCounter.Inc()
		http.Error(w, "Metric name required", http.StatusBadRequest)
		return
	}
	logger := log.With(h.logger, "namespace", namespace, "metric", metricName)

	config, err := configFromQuery(r.URL.Query())
	if err != nil {
		h.errorCounter.Inc()
		http.Error(w, "Invalid query: "+err.Error(), http.StatusBadRequest)
		return
	}
	reporter, err := newReporter(h.logger, config, h.reporterDurationSummary)
	if err != nil {
		h.errorCounter.Inc()
		level.Error(h.logger).Log("msg", "Couldn't create reporter", "err", err.Error())
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	reporter.namespace = namespace   // FIXME
	reporter.metricName = metricName // FIXME
	c := newCollector(logger, reporter, h.errorCounter)

	registry := prometheus.NewRegistry()
	registry.MustRegister(c)
	registry.MustRegister(c.metricsGauge) // FIXME
	promhttp.HandlerFor(registry, promhttp.HandlerOpts{}).ServeHTTP(w, r)
	h.durationSummary.WithLabelValues(namespace, metricName).Observe(time.Since(start).Seconds())
}
