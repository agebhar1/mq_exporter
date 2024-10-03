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

package collector

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	namespace = "mq"
	subsystem = "queue"
)

type Queue struct {
	Metadata QueueMetadata
	Reader   QueueMetricsReader
}

type QueueMetadata struct {
	QueueName      string
	ConnectionName string
	QMgrName       string
	ChannelName    string
}

type QueueMetricsReader interface {
	Read() (QueueMetrics, error)
}

type QueueMetrics struct {
	Metadata        QueueMetadata
	CurrentDepth    int32
	MaxDepth        int32
	OpenInputCount  int32
	OpenOutputCount int32
	RequestDuration time.Duration
}

type QueueCollector struct {
	sync.Mutex
	logger  *slog.Logger
	timeout time.Duration
	queues  []Queue

	up              *prometheus.GaugeVec
	currentDepth    *prometheus.GaugeVec
	maxDepth        *prometheus.GaugeVec
	openInputCount  *prometheus.GaugeVec
	openOutputCount *prometheus.GaugeVec
	requestDuration *prometheus.GaugeVec
}

func (m *QueueMetadata) prometheusLabelValues() []string {
	return []string{
		m.QueueName,
		m.ConnectionName,
		m.QMgrName,
		m.ChannelName,
	}
}

func NewQueueCollector(logger *slog.Logger, timeout time.Duration, queues []Queue) *QueueCollector {

	newQueueMetric := func(name string, help string) *prometheus.GaugeVec {
		return prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      name,
			Help:      help,
		}, []string{"name", "connection", "queue_manager", "channel"})
	}

	return &QueueCollector{
		logger:  logger,
		timeout: timeout,
		queues:  queues,

		up:              newQueueMetric("up", "Was the last scrape of the queue successful."),
		currentDepth:    newQueueMetric("current_depth", "Current number of messages on queue."),
		maxDepth:        newQueueMetric("max_depth", "Maximum number of messages allowed on queue."),
		openInputCount:  newQueueMetric("open_input_count", "Number of MQOPEN calls that have the queue open for input."),
		openOutputCount: newQueueMetric("open_output_count", "Number of MQOPEN calls that have the queue open for output."),
		requestDuration: newQueueMetric("request_duration_seconds", "Duration for request queue metrics in seconds."),
	}
}

func (c *QueueCollector) reset() {
	for _, queue := range c.queues {
		c.up.WithLabelValues(queue.Metadata.prometheusLabelValues()...).Set(0)
	}
	c.currentDepth.Reset()
	c.maxDepth.Reset()
	c.openInputCount.Reset()
	c.openOutputCount.Reset()
	c.requestDuration.Reset()
}

func (c *QueueCollector) Describe(ch chan<- *prometheus.Desc) {
	c.up.Describe(ch)
	c.currentDepth.Describe(ch)
	c.maxDepth.Describe(ch)
	c.openInputCount.Describe(ch)
	c.openOutputCount.Describe(ch)
	c.requestDuration.Describe(ch)
}

func (c *QueueCollector) Collect(ch chan<- prometheus.Metric) {

	c.Lock()
	defer c.Unlock()

	c.reset()

	metrics := collect(c.logger, c.timeout, c.queues, context.Background())
	for _, m := range *metrics {

		lvs := m.Metadata.prometheusLabelValues()

		c.up.WithLabelValues(lvs...).Set(1)
		c.currentDepth.WithLabelValues(lvs...).Set(float64(m.CurrentDepth))
		c.maxDepth.WithLabelValues(lvs...).Set(float64(m.MaxDepth))
		c.openInputCount.WithLabelValues(lvs...).Set(float64(m.OpenInputCount))
		c.openOutputCount.WithLabelValues(lvs...).Set(float64(m.OpenOutputCount))
		c.requestDuration.WithLabelValues(lvs...).Set(float64(m.RequestDuration.Seconds()))
	}

	c.up.Collect(ch)
	c.currentDepth.Collect(ch)
	c.maxDepth.Collect(ch)
	c.openInputCount.Collect(ch)
	c.openOutputCount.Collect(ch)
	c.requestDuration.Collect(ch)
}

func collect(logger *slog.Logger, timeout time.Duration, queues []Queue, ctx context.Context) *[]QueueMetrics {

	metrics := make([]QueueMetrics, 0)

	ctx, cancel := context.WithTimeout(ctx, timeout)

	ch := make(chan QueueMetrics)
	defer close(ch)

	go func() {
		defer cancel()

		for _, queue := range queues {
			metric, err := queue.Reader.Read()
			if ctx.Err() != nil {
				return
			}
			if err == nil {
				ch <- metric
			}
		}
	}()

	for {
		select {
		case metric := <-ch:
			logger.Debug("Got queue metrics", "queue", metric.Metadata.QueueName, "connection", metric.Metadata.ConnectionName, "queue_manager", metric.Metadata.QMgrName, "channel", metric.Metadata.ChannelName)
			metrics = append(metrics, metric)
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				logger.Error("Deadline exceeded while waiting for queue metrics", "timeout", timeout)
			}
			return &metrics
		}
	}
}
