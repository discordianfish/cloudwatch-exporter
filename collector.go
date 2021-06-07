package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stoewer/go-strcase"
)

const batchSize = 500

var (
	// FIXME: technically it may not start with 0-9
	prometheusMetricNameRegexp = regexp.MustCompile("[^a-zA-Z0-9_:]")
)

type collector struct {
	logger log.Logger
	*reporter
	descMap      map[string]*prometheus.Desc
	descLock     sync.Mutex
	metricsDesc  *prometheus.Desc
	metricsSent  uint64
	errorCounter prometheus.Counter
	errDesc      *prometheus.Desc
	concurrency  int
}

func newCollector(logger log.Logger, reporter *reporter, errorCounter prometheus.Counter) *collector {
	return &collector{
		logger:       logger,
		reporter:     reporter,
		descMap:      make(map[string]*prometheus.Desc),
		errDesc:      prometheus.NewDesc("cloudwatch_error", "Error collecting metrics", nil, nil),
		metricsDesc:  prometheus.NewDesc("aws_metrics_sent", "Number of metrics sent in this scrape", nil, nil),
		errorCounter: errorCounter,
		concurrency:  10,
	}
}

// Describe implements Prometheus.Collector.
func (c *collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- prometheus.NewDesc("dummy", "dummy", nil, nil)
}

// Collect implements Prometheus.Collector.
func (c *collector) Collect(ch chan<- prometheus.Metric) {
	metrics, err := c.reporter.ListMetrics()
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to list metrics", "err", err)
		c.errorCounter.Inc()
		ch <- prometheus.NewInvalidMetric(c.errDesc, err)
		return
	}
	level.Debug(c.logger).Log("msg", "list metrics returned", "metrics", metrics)

	// if we have less than batchSize results, we don't want to have zero entries
	length := len(metrics)
	if length > batchSize {
		length = batchSize
	}
	var (
		scratch = make([]types.Metric, length, batchSize)
		i       = 0
		sem     = make(chan bool, c.concurrency)
	)
	for _, metric := range metrics {
		scratch[i] = metric
		i++
		if i < batchSize {
			continue
		}
		i = 0
		batch := make([]types.Metric, length, batchSize)
		sem <- true
		copy(batch, scratch)
		go func(batch []types.Metric) {
			c.collectBatch(ch, batch)
			<-sem
		}(batch)
	}
	// The length of the array might be bigger than the number of entries when processing more than one batch
	batch := make([]types.Metric, length, batchSize)
	sem <- true
	copy(batch, scratch)
	go func(batch []types.Metric) {
		c.collectBatch(ch, batch[:i])
		<-sem
	}(batch)
	for i := 0; i < cap(sem); i++ {
		sem <- true
	}

	ch <- prometheus.MustNewConstMetric(
		c.metricsDesc,
		prometheus.GaugeValue,
		float64(atomic.LoadUint64(&c.metricsSent)),
	)
}

func (c *collector) collectMetric(ch chan<- prometheus.Metric, m *types.Metric, value float64) {
	var (
		namespace = strcase.SnakeCase(prometheusMetricNameRegexp.ReplaceAllString(*m.Namespace, "_"))
		name      = strcase.SnakeCase(prometheusMetricNameRegexp.ReplaceAllString(*m.MetricName, "_"))

		lns = make([]string, len(m.Dimensions))
		lvs = make([]string, len(m.Dimensions))
	)
	// FIXME: do we need to sort the keys?
	for i, d := range m.Dimensions {
		lns[i] = strcase.SnakeCase(*d.Name)
		lvs[i] = *d.Value
	}

	key := strings.Join(lns, " ")
	level.Debug(c.logger).Log("msg", "Using key", "key", key)
	c.descLock.Lock()
	desc, ok := c.descMap[key]
	if !ok {
		level.Debug(c.logger).Log("msg", "Key not found, creating new decs")
		desc = prometheus.NewDesc(namespace+"_"+name, fmt.Sprintf("Cloudwatch Metric %s/%s", *m.Namespace, *m.MetricName), lns, nil)
		c.descMap[key] = desc
	}
	level.Debug(c.logger).Log("msg", "Sending metric", "desc", desc.String(), "lvs", fmt.Sprintf("%+v", lvs), "value", fmt.Sprintf("%f", value))
	ch <- prometheus.MustNewConstMetric(
		desc,
		prometheus.UntypedValue,
		value,
		lvs...,
	)
	c.descLock.Unlock()
	atomic.AddUint64(&c.metricsSent, 1)
}

func sprintDims(ds []types.Dimension) (out string) {
	for _, d := range ds {
		out = fmt.Sprintf("%s%s=%s,", out, *d.Name, *d.Value)
	}
	return out
}

func (c *collector) collectBatch(ch chan<- prometheus.Metric, metrics []types.Metric) {
	// FIXME: API call fails when MetricDataQueries is empty but we might
	// want to avoid that situation in the first place
	if len(metrics) == 0 {
		return
	}
	results, err := c.reporter.GetMetricsResults(metrics)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to get metric results", "err", err)
		c.errorCounter.Inc()
		ch <- prometheus.NewInvalidMetric(c.errDesc, err)
		return
	}
	nr := len(results)
	nm := len(metrics)
	if nr != nm {
		panic(fmt.Sprintf("not same length: %d != %d", nr, nm))
	}
	for _, result := range results {
		// idx is index in batch
		idx, err := strconv.Atoi((*result.Id)[1:]) // strip "n" prefix
		if err != nil {
			panic(err)
		}
		level.Debug(c.logger).Log("id", *result.Id)
		m := metrics[idx]
		level.Debug(c.logger).Log("msg", "creating metric", "index", idx, "dimensions", sprintDims(m.Dimensions))
		if len(result.Values) == 0 {
			level.Debug(c.logger).Log("msg", "no values found")
			continue
		}
		c.collectMetric(ch, &m, result.Values[0])
	}
}
