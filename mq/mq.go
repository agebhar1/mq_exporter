// Copyright 2021-2026 Andreas Gebhardt
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

package mq

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/agebhar1/mq_exporter/collector"
	"github.com/ibm-messaging/mq-golang/v5/ibmmq"
	"gopkg.in/yaml.v2"
)

var (
	defaultTimeout = 3 * time.Second

	selectors = []int32{
		ibmmq.MQCA_Q_NAME,
		ibmmq.MQIA_MAX_Q_DEPTH,
		ibmmq.MQIA_CURRENT_Q_DEPTH,
		ibmmq.MQIA_OPEN_INPUT_COUNT,
		ibmmq.MQIA_OPEN_OUTPUT_COUNT,
	}
)

const (
	YES = 1
	NO  = 0
)

type MqConfiguration struct {
	QueueManager  string `yaml:"queueManager"`
	User          string
	Password      string
	ConnName      string `yaml:"connName"`
	Channel       string
	SSLCipherSpec string `yaml:"sslCipherSpec"`
	KeyRepository string `yaml:"keyRepository"`
	Timeout       *time.Duration
	Queues        []string
}

func readConfigYaml(filename string) (*MqConfiguration, error) {

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("configuration file '%s' does not exists or is not readable", filename)
	}

	var cfg MqConfiguration

	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}

	if cfg.Timeout == nil {
		cfg.Timeout = &defaultTimeout
	}

	return &cfg, nil
}

func (cfg *MqConfiguration) validateReadFromYaml() error {

	missingMandatoryFields := make([]string, 0, 4)

	if cfg.QueueManager == "" {
		missingMandatoryFields = append(missingMandatoryFields, "'queueManager'")
	}
	if cfg.ConnName == "" {
		missingMandatoryFields = append(missingMandatoryFields, "'connName'")
	}
	if cfg.Channel == "" {
		missingMandatoryFields = append(missingMandatoryFields, "'channel'")
	}

	if len(missingMandatoryFields) > 0 {
		return fmt.Errorf("missing mandatory fields: %s", strings.Join(missingMandatoryFields, ", "))
	}

	if cfg.User == "" && cfg.Password != "" || (cfg.User != "" && cfg.Password == "") {
		return fmt.Errorf("requires both 'user' and 'password'")
	}
	if cfg.SSLCipherSpec == "" && cfg.KeyRepository != "" || (cfg.SSLCipherSpec != "" && cfg.KeyRepository == "") {
		return fmt.Errorf("requires both 'sslCipherSpec' and 'keyRepository'")
	}

	if cfg.Timeout == nil || cfg.Timeout.Milliseconds() <= 0 {
		return fmt.Errorf("requires strict positive 'timeout'")
	}

	return nil
}

type MqConnection struct {
	isConnecting *int64
	cfg          *MqConfiguration
	logger       *slog.Logger
	qMgr         ibmmq.MQQueueManager
	queues       map[string]ibmmq.MQObject
}

func NewMqConnection(logger *slog.Logger, cfgFilename string) (*MqConnection, error) {

	cfg, err := readConfigYaml(cfgFilename)
	if err != nil {
		return nil, err
	}
	if err := cfg.validateReadFromYaml(); err != nil {
		return nil, err
	}

	c := MqConnection{
		isConnecting: new(int64),
		cfg:          cfg,
		logger:       logger.With("connName", cfg.ConnName, "channel", cfg.Channel, "queueManager", cfg.QueueManager),
	}
	*c.isConnecting = NO

	err = c.connect()
	if err != nil {
		return nil, err
	}

	return &c, nil
}

func (c *MqConnection) connect() error {

	if !atomic.CompareAndSwapInt64(c.isConnecting, NO, YES) {
		return fmt.Errorf("connect still in progress")
	}
	defer func() {
		atomic.StoreInt64(c.isConnecting, NO)
		c.logger.Info("connected to queue manager")
	}()

	if len(c.cfg.Queues) > 0 {

		cd := ibmmq.NewMQCD()
		cd.ChannelName = c.cfg.Channel
		cd.ConnectionName = c.cfg.ConnName

		cno := ibmmq.NewMQCNO()
		cno.ClientConn = cd
		cno.Options = ibmmq.MQCNO_CLIENT_BINDING

		if c.cfg.User != "" {
			csp := ibmmq.NewMQCSP()
			csp.AuthenticationType = ibmmq.MQCSP_AUTH_USER_ID_AND_PWD
			csp.UserId = c.cfg.User
			csp.Password = c.cfg.Password

			cno.SecurityParms = csp
		}

		if c.cfg.SSLCipherSpec != "" {
			cd.SSLCipherSpec = c.cfg.SSLCipherSpec
			cd.SSLClientAuth = ibmmq.MQSCA_OPTIONAL

			sco := ibmmq.NewMQSCO()
			sco.KeyRepository = c.cfg.KeyRepository

			cno.SSLConfig = sco
		}

		qMgr, err := ibmmq.Connx(c.cfg.QueueManager, cno)
		if err != nil {
			return err
		}
		c.qMgr = qMgr

		c.queues = make(map[string]ibmmq.MQObject)
		for _, qName := range c.cfg.Queues {
			od := ibmmq.NewMQOD()
			od.ObjectType = ibmmq.MQOT_Q
			od.ObjectName = qName
			queue, err := qMgr.Open(od, ibmmq.MQOO_INQUIRE)
			if err != nil {
				return err
			}
			c.queues[qName] = queue
		}
	}
	return nil
}

func (c *MqConnection) handleReturnValue(mqret *ibmmq.MQReturn) {
	if mqret.MQCC == ibmmq.MQCC_FAILED && mqret.MQRC == ibmmq.MQRC_CONNECTION_BROKEN {
		go func() {
			err := c.connect()
			if err != nil {
				c.logger.Error("failed re-connect", "err", err)
			}
		}()
	}
	// syscall.Kill(syscall.Getpid(), syscall.SIGINT)
}

func (c *MqConnection) resolveQueue(q *MqQueue) ibmmq.MQObject {
	return c.queues[q.metadata.QueueName]
}

func (c *MqConnection) inqQueue(q *MqQueue, goSelectors []int32) (map[int32]interface{}, error) {
	values, err := c.resolveQueue(q).Inq(goSelectors)
	if err != nil {
		go c.handleReturnValue(err.(*ibmmq.MQReturn))
	}
	return values, err
}

func (c *MqConnection) Queues() []collector.Queue {
	xs := make([]collector.Queue, 0)
	for queue := range c.queues {
		metadata := collector.QueueMetadata{
			QueueName:      queue,
			ConnectionName: c.cfg.ConnName,
			QMgrName:       c.cfg.QueueManager,
			ChannelName:    c.cfg.Channel,
		}
		xs = append(xs, collector.Queue{
			Metadata: metadata,
			Reader: &MqQueue{
				connection: c,
				logger:     c.logger.With("queue", queue),
				metadata:   metadata,
			},
		})
	}
	return xs
}

func (c *MqConnection) Close() {
	for _, queue := range c.queues {
		err := queue.Close(0)
		if err == nil {
			c.logger.Info("closed queue", "queue", queue.Name)
		} else {
			c.logger.Error("failed to close queue", "err", err, "queue", queue.Name)
		}
	}
	err := c.qMgr.Disc()
	if err == nil {
		c.logger.Info("disconnected from queue manager")
	} else {
		c.logger.Error("failed to disconnect from queue manager", "err", err)
	}
}

func (c *MqConnection) Timeout() time.Duration {
	return *c.cfg.Timeout
}

type MqQueue struct {
	connection *MqConnection
	logger     *slog.Logger
	metadata   collector.QueueMetadata
}

func (q *MqQueue) Read() (collector.QueueMetrics, error) {
	start := time.Now()
	values, err := q.connection.inqQueue(q, selectors)
	if err != nil {
		err := err.(*ibmmq.MQReturn)
		q.logger.Error("error inquire queue", "err", err, "mqcc", err.MQCC, "mqcr", err.MQRC)
		return collector.QueueMetrics{}, err
	}
	return collector.QueueMetrics{
		Metadata:        q.metadata,
		MaxDepth:        values[ibmmq.MQIA_MAX_Q_DEPTH].(int32),
		CurrentDepth:    values[ibmmq.MQIA_CURRENT_Q_DEPTH].(int32),
		OpenInputCount:  values[ibmmq.MQIA_OPEN_INPUT_COUNT].(int32),
		OpenOutputCount: values[ibmmq.MQIA_OPEN_OUTPUT_COUNT].(int32),
		RequestDuration: time.Since(start),
	}, nil
}
