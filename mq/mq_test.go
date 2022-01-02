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

package mq

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"gotest.tools/v3/assert"
)

var fixturesPath = "fixtures"

func TestReadConfig_Full(t *testing.T) {

	got, err := readConfigYaml(filepath.Join(fixturesPath, "config-full.yaml"))
	if err != nil {
		t.Error(err)
	}

	timeout := 1500 * time.Millisecond

	want := &MqConfiguration{
		QueueManager:  "QM1",
		User:          "app",
		Password:      "passw0rd",
		ConnName:      "localhost(1414)",
		Channel:       "DEV.APP.SVRCONN",
		SSLCipherSpec: "TLS_RSA_WITH_AES_128_CBC_SHA256",
		KeyRepository: "./",
		Timeout:       &timeout,
		Queues:        []string{"DEV.QUEUE.1", "DEV.QUEUE.2", "DEV.QUEUE.3"},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Should contain expected configuration (-want, +got):\n%s", diff)
	}
}

func TestReadConfig_DefaultValues(t *testing.T) {

	got, err := readConfigYaml(filepath.Join(fixturesPath, "config-empty.yaml"))
	if err != nil {
		t.Error(err)
	}

	want := &MqConfiguration{
		Timeout: &defaultTimeout,
	}

	assert.Equal(t, defaultTimeout, 3*time.Second)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Should contain expected default values (-want, +got):\n%s", diff)
	}
}

func TestReadConfig_NonExisting(t *testing.T) {

	_, err := readConfigYaml(filepath.Join(fixturesPath, "does-not-exists.yaml"))
	assert.Error(t, err, "configuration file 'fixtures/does-not-exists.yaml' does not exists or is not readable")
}

func TestValidate(t *testing.T) {

	type args struct {
		cfg *MqConfiguration
	}

	zero := 0 * time.Second

	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "missing mandatory fields",
			args: args{
				cfg: &MqConfiguration{},
			},
			want: "missing mandatory fields: 'queueManager', 'connName', 'channel'",
		},
		{
			name: "requires password if user is provided",
			args: args{
				cfg: &MqConfiguration{
					QueueManager: "QM1",
					User:         "app",
					ConnName:     "localhost(1414)",
					Channel:      "DEV.APP.SVRCONN",
				},
			},
			want: "requires both 'user' and 'password'",
		},
		{
			name: "requires user if password is provided",
			args: args{
				cfg: &MqConfiguration{
					QueueManager: "QM1",
					Password:     "passw0rd",
					ConnName:     "localhost(1414)",
					Channel:      "DEV.APP.SVRCONN",
				},
			},
			want: "requires both 'user' and 'password'",
		},
		{
			name: "requires keyRepository if sslCipherSpec is provided",
			args: args{
				cfg: &MqConfiguration{
					QueueManager:  "QM1",
					ConnName:      "localhost(1414)",
					Channel:       "DEV.APP.SVRCONN",
					SSLCipherSpec: "TLS_RSA_WITH_AES_128_CBC_SHA256",
				},
			},
			want: "requires both 'sslCipherSpec' and 'keyRepository'",
		},
		{
			name: "requires sslCipherSpec if keyRepository is provided",
			args: args{
				cfg: &MqConfiguration{
					QueueManager:  "QM1",
					ConnName:      "localhost(1414)",
					Channel:       "DEV.APP.SVRCONN",
					SSLCipherSpec: "TLS_RSA_WITH_AES_128_CBC_SHA256",
				},
			},
			want: "requires both 'sslCipherSpec' and 'keyRepository'",
		},
		{
			name: "requires strict positive timeout",
			args: args{
				cfg: &MqConfiguration{
					QueueManager: "QM1",
					ConnName:     "localhost(1414)",
					Channel:      "DEV.APP.SVRCONN",
					Timeout:      &zero,
				},
			},
			want: "requires strict positive 'timeout'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			err := tt.args.cfg.validateReadFromYaml()
			if err == nil {
				t.Error("Expect error due to incomplete/faulty configuration.")
			}
			assert.Error(t, err, tt.want)

		})
	}
}
