package main

import (
	"net/http"
	"os"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promlog"
	"github.com/prometheus/common/promlog/flag"
	"github.com/prometheus/common/version"
	"github.com/prometheus/exporter-toolkit/web"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

// promLogger implements promhttp.Logger
type promLogger struct {
	log.Logger
}

func (l promLogger) Println(v ...interface{}) {
	level.Error(l.Logger).Log(v...)
}

// promLogger implements the Logger interface
func (l promLogger) Log(v ...interface{}) error {
	level.Info(l.Logger).Log(v...)
	return nil
}

func main() {
	var (
		listenAddress = kingpin.Flag(
			"web.listen-address",
			"Address on which to expose metrics and web interface.",
		).Default(":9106").String()
		metricsPath = kingpin.Flag(
			"web.path",
			"Path prefix under which to expose metrics.",
		).Default("/metrics/").String()
		telemetryListenAddress = kingpin.Flag(
			"web.telemetry-listen-address",
			"Address on which to expose exporter internal metrics.",
		).Default(":8080").String()
		telemetryMetricsPath = kingpin.Flag(
			"web.telemetry-path",
			"Path prefix under which to expose metrics.",
		).Default("/metrics").String()
		tlsConfig = kingpin.Flag(
			"web.config",
			"[EXPERIMENTAL] Path to config yaml file that can enable TLS or authentication.",
		).Default("").String()

		durationSummary = prometheus.NewSummaryVec(prometheus.SummaryOpts{
			Name: "cloudwatch_request_duration_seconds",
			Help: "Duration of cloudwatch metric collection.",
		}, []string{"namespace", "name"})
		errorCounter = prometheus.NewCounter(prometheus.CounterOpts{
			Name: "cloudwatch_errors_total",
			Help: "Number of errors.",
		})
	)
	promlogConfig := &promlog.Config{}
	flag.AddFlags(kingpin.CommandLine, promlogConfig)
	kingpin.Version(version.Print("cloudwatch_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()
	logger := promlog.New(promlogConfig)

	registry := prometheus.NewRegistry()
	registry.MustRegister(durationSummary)
	registry.MustRegister(errorCounter)

	var (
		telemetryMux    = http.NewServeMux()
		telemetryServer = http.Server{Handler: telemetryMux, Addr: *telemetryListenAddress}
	)
	telemetryMux.Handle(*telemetryMetricsPath, promhttp.HandlerFor(registry, promhttp.HandlerOpts{ErrorLog: promLogger{logger}}))

	var (
		metricsMux    = http.NewServeMux()
		metricsServer = http.Server{Handler: metricsMux, Addr: *listenAddress}
	)
	metricsMux.Handle(*metricsPath, newHandler(logger, *metricsPath, durationSummary, errorCounter))
	metricsMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
			<head><title>Cloudwatch Exporter</title></head>
			<body>
			<h1>Cloudwatch Exporter</h1>
			</body>
			</html>`))
	})

	go func() {
		level.Info(logger).Log("msg", "Listening for internal telemetry requests on", "address", *telemetryListenAddress)
		if err := web.ListenAndServe(&telemetryServer, *tlsConfig, logger); err != nil {
			level.Error(logger).Log("err", err)
			os.Exit(1)
		}
	}()

	level.Info(logger).Log("msg", "Listening for cloudwatch metric requests on", "address", *listenAddress)
	if err := web.ListenAndServe(&metricsServer, *tlsConfig, logger); err != nil {
		level.Error(logger).Log("err", err)
		os.Exit(1)
	}
}
