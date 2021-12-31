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
	"errors"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/google/go-cmp/cmp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

var logger = log.NewNopLogger()

type succeedingQueueMetricReader struct {
	value QueueMetrics
}

func (r succeedingQueueMetricReader) Read() (QueueMetrics, error) {
	return r.value, nil
}

type failingQueueMetricReader struct {
	value error
}

func (r failingQueueMetricReader) Read() (QueueMetrics, error) {
	return QueueMetrics{}, r.value
}

type slowQueueMetricReader struct {
	duration time.Duration
	value    QueueMetrics
}

func (r slowQueueMetricReader) Read() (QueueMetrics, error) {
	time.Sleep(r.duration)
	return r.value, nil
}

func (m QueueMetadata) succeeding() Queue {
	return Queue{Metadata: m, Reader: succeedingQueueMetricReader{value: QueueMetrics{Metadata: m}}}
}

func (m QueueMetadata) succeedingWith(value QueueMetrics) Queue {
	value.Metadata = m
	return Queue{Metadata: m, Reader: succeedingQueueMetricReader{value: value}}
}

func (m QueueMetadata) failingWith(value error) Queue {
	return Queue{Metadata: m, Reader: failingQueueMetricReader{value: value}}
}

func (m QueueMetadata) slowBy(duration time.Duration) Queue {
	return Queue{Metadata: m, Reader: slowQueueMetricReader{duration: duration, value: QueueMetrics{Metadata: m}}}
}

func TestCollectMetrics(t *testing.T) {

	type args struct {
		queues  []Queue
		timeout time.Duration
	}

	q1 := QueueMetadata{QueueName: "DEV.QUEUE.1"}
	q2 := QueueMetadata{QueueName: "DEV.QUEUE.2"}
	q3 := QueueMetadata{QueueName: "DEV.QUEUE.3"}

	tests := []struct {
		name string
		args args
		want []QueueMetrics
	}{
		{
			name: "no reads (reader)",
			args: args{
				queues:  []Queue{},
				timeout: time.Minute,
			},
			want: []QueueMetrics{},
		},
		{
			name: "single succeeding read",
			args: args{
				queues: []Queue{
					q1.succeeding(),
				},
				timeout: time.Minute,
			},
			want: []QueueMetrics{{Metadata: q1}},
		},
		{
			name: "multiple succeeding reads",
			args: args{
				queues: []Queue{
					q1.succeeding(),
					q2.succeeding(),
				},
				timeout: time.Minute,
			},
			want: []QueueMetrics{{Metadata: q1}, {Metadata: q2}},
		},
		{
			name: "single failing read",
			args: args{
				queues:  []Queue{q1.failingWith(errors.New("Failed"))},
				timeout: time.Minute},
			want: []QueueMetrics{},
		},
		{
			name: "skip failing read(s)",
			args: args{
				queues: []Queue{
					q1.failingWith(errors.New("Failed")),
					q2.succeeding(),
					q3.failingWith(errors.New("Failed")),
				},
				timeout: time.Minute},
			want: []QueueMetrics{{Metadata: q2}},
		},
		{
			name: "single timeout read",
			args: args{
				queues: []Queue{
					q1.slowBy(1 * time.Minute),
				},
				timeout: 500 * time.Millisecond,
			},
			want: []QueueMetrics{},
		},
		{
			name: "skip read after timeout",
			args: args{
				queues: []Queue{
					q1.succeeding(),
					q2.slowBy(1 * time.Minute),
					q3.succeeding(),
				},
				timeout: 500 * time.Millisecond,
			},
			want: []QueueMetrics{{Metadata: q1}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			have := collect(logger, tt.args.timeout, tt.args.queues, context.Background())

			if diff := cmp.Diff(tt.want, *have); diff != "" {
				t.Errorf("Should contain expected metric(s) (-want, +got):\n%s", diff)
			}

		})
	}
}

func TestCollectDoesNotLeakGoRoutine(t *testing.T) {

	numGoroutinesBefore := runtime.NumGoroutine()

	q1 := QueueMetadata{QueueName: "DEV.QUEUE.1", ConnectionName: "localhost(1414)", QMgrName: "QM1", ChannelName: "DEV.APP.SVRCONN"}
	q2 := QueueMetadata{QueueName: "DEV.QUEUE.2", ConnectionName: "localhost(1414)", QMgrName: "QM1", ChannelName: "DEV.APP.SVRCONN"}

	queues := []Queue{
		q1.slowBy(2 * time.Second),
		q2.succeeding(),
	}

	collect(logger, 500*time.Millisecond, queues, context.Background())

	time.Sleep(3 * time.Second)
	if numGoroutinesAfter := runtime.NumGoroutine(); numGoroutinesAfter > numGoroutinesBefore {
		t.Fatalf("Should not leak go routine: %d (before), %d (after).", numGoroutinesBefore, numGoroutinesAfter)
	}
}

func TestCollectorAllQueueRequestsSucceeds(t *testing.T) {

	testcase := `# HELP mq_queue_current_depth Current number of messages on queue.
# TYPE mq_queue_current_depth gauge
mq_queue_current_depth{channel="DEV.APP.SVRCONN",connection="localhost(1414)",name="DEV.QUEUE.1",queue_manager="QM1"} 1
mq_queue_current_depth{channel="DEV.APP.SVRCONN",connection="localhost(1414)",name="DEV.QUEUE.2",queue_manager="QM1"} 0
# HELP mq_queue_max_depth Maximum number of messages allowed on queue.
# TYPE mq_queue_max_depth gauge
mq_queue_max_depth{channel="DEV.APP.SVRCONN",connection="localhost(1414)",name="DEV.QUEUE.1",queue_manager="QM1"} 500
mq_queue_max_depth{channel="DEV.APP.SVRCONN",connection="localhost(1414)",name="DEV.QUEUE.2",queue_manager="QM1"} 500
# HELP mq_queue_open_input_count Number of MQOPEN calls that have the queue open for input.
# TYPE mq_queue_open_input_count gauge
mq_queue_open_input_count{channel="DEV.APP.SVRCONN",connection="localhost(1414)",name="DEV.QUEUE.1",queue_manager="QM1"} 0
mq_queue_open_input_count{channel="DEV.APP.SVRCONN",connection="localhost(1414)",name="DEV.QUEUE.2",queue_manager="QM1"} 1
# HELP mq_queue_open_output_count Number of MQOPEN calls that have the queue open for output.
# TYPE mq_queue_open_output_count gauge
mq_queue_open_output_count{channel="DEV.APP.SVRCONN",connection="localhost(1414)",name="DEV.QUEUE.1",queue_manager="QM1"} 1
mq_queue_open_output_count{channel="DEV.APP.SVRCONN",connection="localhost(1414)",name="DEV.QUEUE.2",queue_manager="QM1"} 0
# HELP mq_queue_request_duration_seconds Duration for request queue metrics in seconds.
# TYPE mq_queue_request_duration_seconds gauge
mq_queue_request_duration_seconds{channel="DEV.APP.SVRCONN",connection="localhost(1414)",name="DEV.QUEUE.1",queue_manager="QM1"} 0.000422679
mq_queue_request_duration_seconds{channel="DEV.APP.SVRCONN",connection="localhost(1414)",name="DEV.QUEUE.2",queue_manager="QM1"} 0.000335981
# HELP mq_queue_up Was the last scrape of the queue successful.
# TYPE mq_queue_up gauge
mq_queue_up{channel="DEV.APP.SVRCONN",connection="localhost(1414)",name="DEV.QUEUE.1",queue_manager="QM1"} 1
mq_queue_up{channel="DEV.APP.SVRCONN",connection="localhost(1414)",name="DEV.QUEUE.2",queue_manager="QM1"} 1
`
	q1 := QueueMetadata{QueueName: "DEV.QUEUE.1", ConnectionName: "localhost(1414)", QMgrName: "QM1", ChannelName: "DEV.APP.SVRCONN"}
	q2 := QueueMetadata{QueueName: "DEV.QUEUE.2", ConnectionName: "localhost(1414)", QMgrName: "QM1", ChannelName: "DEV.APP.SVRCONN"}

	queues := []Queue{
		q1.succeedingWith(
			QueueMetrics{
				CurrentDepth:    1,
				MaxDepth:        500,
				OpenInputCount:  0,
				OpenOutputCount: 1,
				RequestDuration: 422679 * time.Nanosecond,
			}),
		q2.succeedingWith(
			QueueMetrics{
				CurrentDepth:    0,
				MaxDepth:        500,
				OpenInputCount:  1,
				OpenOutputCount: 0,
				RequestDuration: 335981 * time.Nanosecond,
			}),
	}

	collector := NewQueueCollector(logger, 1*time.Second, queues)

	reg := prometheus.NewRegistry()
	reg.MustRegister(collector)

	err := testutil.GatherAndCompare(reg, strings.NewReader(testcase))
	if err != nil {
		t.Fatal(err)
	}
}

func TestCollectorWithQueueRequestTimeout(t *testing.T) {

	testcase := `# HELP mq_queue_current_depth Current number of messages on queue.
# TYPE mq_queue_current_depth gauge
mq_queue_current_depth{channel="DEV.APP.SVRCONN",connection="localhost(1414)",name="DEV.QUEUE.1",queue_manager="QM1"} 1
# HELP mq_queue_max_depth Maximum number of messages allowed on queue.
# TYPE mq_queue_max_depth gauge
mq_queue_max_depth{channel="DEV.APP.SVRCONN",connection="localhost(1414)",name="DEV.QUEUE.1",queue_manager="QM1"} 500
# HELP mq_queue_open_input_count Number of MQOPEN calls that have the queue open for input.
# TYPE mq_queue_open_input_count gauge
mq_queue_open_input_count{channel="DEV.APP.SVRCONN",connection="localhost(1414)",name="DEV.QUEUE.1",queue_manager="QM1"} 0
# HELP mq_queue_open_output_count Number of MQOPEN calls that have the queue open for output.
# TYPE mq_queue_open_output_count gauge
mq_queue_open_output_count{channel="DEV.APP.SVRCONN",connection="localhost(1414)",name="DEV.QUEUE.1",queue_manager="QM1"} 1
# HELP mq_queue_request_duration_seconds Duration for request queue metrics in seconds.
# TYPE mq_queue_request_duration_seconds gauge
mq_queue_request_duration_seconds{channel="DEV.APP.SVRCONN",connection="localhost(1414)",name="DEV.QUEUE.1",queue_manager="QM1"} 0.000422679
# HELP mq_queue_up Was the last scrape of the queue successful.
# TYPE mq_queue_up gauge
mq_queue_up{channel="DEV.APP.SVRCONN",connection="localhost(1414)",name="DEV.QUEUE.1",queue_manager="QM1"} 1
mq_queue_up{channel="DEV.APP.SVRCONN",connection="localhost(1414)",name="DEV.QUEUE.2",queue_manager="QM1"} 0
mq_queue_up{channel="DEV.APP.SVRCONN",connection="localhost(1414)",name="DEV.QUEUE.3",queue_manager="QM1"} 0
`

	q1 := QueueMetadata{QueueName: "DEV.QUEUE.1", ConnectionName: "localhost(1414)", QMgrName: "QM1", ChannelName: "DEV.APP.SVRCONN"}
	q2 := QueueMetadata{QueueName: "DEV.QUEUE.2", ConnectionName: "localhost(1414)", QMgrName: "QM1", ChannelName: "DEV.APP.SVRCONN"}
	q3 := QueueMetadata{QueueName: "DEV.QUEUE.3", ConnectionName: "localhost(1414)", QMgrName: "QM1", ChannelName: "DEV.APP.SVRCONN"}

	queues := []Queue{
		q1.succeedingWith(QueueMetrics{
			CurrentDepth:    1,
			MaxDepth:        500,
			OpenInputCount:  0,
			OpenOutputCount: 1,
			RequestDuration: 422679 * time.Nanosecond,
		}),
		q2.slowBy(1 * time.Second),
		q3.succeedingWith(QueueMetrics{
			CurrentDepth:    1,
			MaxDepth:        500,
			OpenInputCount:  0,
			OpenOutputCount: 1,
		}),
	}

	collector := NewQueueCollector(logger, 500*time.Millisecond, queues)

	reg := prometheus.NewRegistry()
	reg.MustRegister(collector)

	err := testutil.GatherAndCompare(reg, strings.NewReader(testcase))
	if err != nil {
		t.Fatal(err)
	}
}

func TestCollectorWithQueueRequestError(t *testing.T) {

	testcase := `# HELP mq_queue_current_depth Current number of messages on queue.
# TYPE mq_queue_current_depth gauge
mq_queue_current_depth{channel="DEV.APP.SVRCONN",connection="localhost(1414)",name="DEV.QUEUE.1",queue_manager="QM1"} 1
mq_queue_current_depth{channel="DEV.APP.SVRCONN",connection="localhost(1414)",name="DEV.QUEUE.3",queue_manager="QM1"} 0
# HELP mq_queue_max_depth Maximum number of messages allowed on queue.
# TYPE mq_queue_max_depth gauge
mq_queue_max_depth{channel="DEV.APP.SVRCONN",connection="localhost(1414)",name="DEV.QUEUE.1",queue_manager="QM1"} 500
mq_queue_max_depth{channel="DEV.APP.SVRCONN",connection="localhost(1414)",name="DEV.QUEUE.3",queue_manager="QM1"} 500
# HELP mq_queue_open_input_count Number of MQOPEN calls that have the queue open for input.
# TYPE mq_queue_open_input_count gauge
mq_queue_open_input_count{channel="DEV.APP.SVRCONN",connection="localhost(1414)",name="DEV.QUEUE.1",queue_manager="QM1"} 0
mq_queue_open_input_count{channel="DEV.APP.SVRCONN",connection="localhost(1414)",name="DEV.QUEUE.3",queue_manager="QM1"} 1
# HELP mq_queue_open_output_count Number of MQOPEN calls that have the queue open for output.
# TYPE mq_queue_open_output_count gauge
mq_queue_open_output_count{channel="DEV.APP.SVRCONN",connection="localhost(1414)",name="DEV.QUEUE.1",queue_manager="QM1"} 1
mq_queue_open_output_count{channel="DEV.APP.SVRCONN",connection="localhost(1414)",name="DEV.QUEUE.3",queue_manager="QM1"} 0
# HELP mq_queue_request_duration_seconds Duration for request queue metrics in seconds.
# TYPE mq_queue_request_duration_seconds gauge
mq_queue_request_duration_seconds{channel="DEV.APP.SVRCONN",connection="localhost(1414)",name="DEV.QUEUE.1",queue_manager="QM1"} 0.000646478
mq_queue_request_duration_seconds{channel="DEV.APP.SVRCONN",connection="localhost(1414)",name="DEV.QUEUE.3",queue_manager="QM1"} 0.000272913
# HELP mq_queue_up Was the last scrape of the queue successful.
# TYPE mq_queue_up gauge
mq_queue_up{channel="DEV.APP.SVRCONN",connection="localhost(1414)",name="DEV.QUEUE.1",queue_manager="QM1"} 1
mq_queue_up{channel="DEV.APP.SVRCONN",connection="localhost(1414)",name="DEV.QUEUE.2",queue_manager="QM1"} 0
mq_queue_up{channel="DEV.APP.SVRCONN",connection="localhost(1414)",name="DEV.QUEUE.3",queue_manager="QM1"} 1
`

	q1 := QueueMetadata{QueueName: "DEV.QUEUE.1", ConnectionName: "localhost(1414)", QMgrName: "QM1", ChannelName: "DEV.APP.SVRCONN"}
	q2 := QueueMetadata{QueueName: "DEV.QUEUE.2", ConnectionName: "localhost(1414)", QMgrName: "QM1", ChannelName: "DEV.APP.SVRCONN"}
	q3 := QueueMetadata{QueueName: "DEV.QUEUE.3", ConnectionName: "localhost(1414)", QMgrName: "QM1", ChannelName: "DEV.APP.SVRCONN"}

	queues := []Queue{
		q1.succeedingWith(QueueMetrics{
			CurrentDepth:    1,
			MaxDepth:        500,
			OpenInputCount:  0,
			OpenOutputCount: 1,
			RequestDuration: 646478 * time.Nanosecond,
		}),
		q2.failingWith(errors.New("Failed")),
		q3.succeedingWith(QueueMetrics{
			CurrentDepth:    0,
			MaxDepth:        500,
			OpenInputCount:  1,
			OpenOutputCount: 0,
			RequestDuration: 272913 * time.Nanosecond,
		}),
	}

	collector := NewQueueCollector(logger, 1*time.Second, queues)

	reg := prometheus.NewRegistry()
	reg.MustRegister(collector)

	err := testutil.GatherAndCompare(reg, strings.NewReader(testcase))
	if err != nil {
		t.Fatal(err)
	}
}
