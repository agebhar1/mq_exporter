// Copyright 2021-2022 Andreas Gebhardt
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	versionc "github.com/prometheus/client_golang/prometheus/collectors/version"
	"github.com/prometheus/common/promslog"
	"github.com/prometheus/common/promslog/flag"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/agebhar1/mq_exporter/collector"
	"github.com/agebhar1/mq_exporter/mq"
	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/version"
	"github.com/prometheus/exporter-toolkit/web"
	webflag "github.com/prometheus/exporter-toolkit/web/kingpinflag"
)

var name = "mq_exporter"

type appCtx struct {
	logger *slog.Logger
	sigs   chan os.Signal

	configFile       *string
	toolkitFlags     *web.FlagConfig
	webTelemetryPath *string
}

func newAppCtx(args []string, usageWriter io.Writer, errorWriter io.Writer, logger *slog.Logger) *appCtx {

	ctx := appCtx{}

	var app = kingpin.New(name, "A Prometheus exporter for MQ metrics.")
	ctx.configFile = app.Flag("config", "Path to config yaml file for MQ connections.").Required().String()
	ctx.toolkitFlags = webflag.AddFlags(app, ":9873")
	ctx.webTelemetryPath = app.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()

	app.UsageWriter(usageWriter)
	app.ErrorWriter(errorWriter)
	app.Version(version.Print(app.Name))
	app.HelpFlag.Short('h')
	app.VersionFlag.Short('v')

	promslogConfig := &promslog.Config{Style: promslog.GoKitStyle}
	flag.AddFlags(app, promslogConfig)

	kingpin.MustParse(app.Parse(args))

	if logger != nil {
		ctx.logger = logger
	} else {
		ctx.logger = promslog.New(promslogConfig)
	}

	ctx.sigs = make(chan os.Signal)
	signal.Notify(ctx.sigs, syscall.SIGINT, syscall.SIGTERM)

	return &ctx
}

func (app *appCtx) run() int {

	app.logger.Info("Starting", "app_name", name, "version", version.Version, "branch", version.Branch, "revision", version.Revision)
	app.logger.Info("Build context", "go", version.GoVersion, "build_user", version.BuildUser, "build_date", version.BuildDate)

	reg := prometheus.NewRegistry()
	reg.MustRegister(versionc.NewCollector(name))
	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	mqConnection, err := mq.NewMqConnection(app.logger, *app.configFile)
	if err != nil {
		app.logger.Error(err.Error())
		return 1
	}

	collector := collector.NewQueueCollector(app.logger, mqConnection.Timeout(), mqConnection.Queues())
	reg.MustRegister(collector)

	handler := http.NewServeMux()
	handler.Handle(*app.webTelemetryPath, promhttp.InstrumentMetricHandler(
		reg, promhttp.HandlerFor(reg, promhttp.HandlerOpts{}),
	))
	handler.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`<html>
			<head><title>MQ Exporter</title></head>
			<body>
			<h1>MQ Exporter</h1>
			<p><a href='` + *app.webTelemetryPath + `'>Metrics</a></p>
			</body>
			</html>`))
	})

	server := &http.Server{Handler: handler}

	go func() {
		<-app.sigs

		mqConnection.Close()

		app.logger.Info("Shutdown server.")
		server.Shutdown(context.Background())
	}()

	if err := web.ListenAndServe(server, app.toolkitFlags, app.logger); err != http.ErrServerClosed {
		app.logger.Error("Serve error", "err", err)
		return 2
	}
	return 0
}

func main() {
	os.Exit(newAppCtx(os.Args[1:], os.Stdout, os.Stderr, nil).run())
}
