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
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"
)

var configArg = "--config=fixtures/config-no-queues.yaml"

type listenAddrListener struct {
	c chan (string)
}

func (l listenAddrListener) Log(keyvals ...interface{}) error {
	if len(keyvals) == 6 && keyvals[3].(string) == "Listening on" {
		addr := keyvals[5].(string)
		l.c <- addr
	}
	return nil
}

func (l listenAddrListener) addr() string {
	return <-l.c
}

func (l listenAddrListener) close() {
	close(l.c)
}

func newListenAddrListener() listenAddrListener {
	return listenAddrListener{c: make(chan string, 1)}
}

func TestDefaultMetricsEndpoint(t *testing.T) {

	l := newListenAddrListener()
	defer l.close()

	app := newAppCtx([]string{"--web.listen-address=:0", configArg}, os.Stdout, os.Stderr, l)

	go app.run()

	resp, err := http.Get("http://" + l.addr() + "/metrics")
	if err != nil {
		t.Error(err)
	}

	defer resp.Body.Close()

	statusCode := 200
	if resp.StatusCode != statusCode {
		t.Log("expected:", statusCode)
		t.Log("     got:", resp.StatusCode)
		t.Error("HTTP status code does not match.")
	}

	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Error(err)
	}

	body := string(responseBody)

	if !strings.Contains(body, "# HELP promhttp_metric_handler_requests_total") {
		t.Errorf("Want response body to contains '# HELP promhttp_metric_handler_requests_total'. But found none in:\n%s", body)
	}

	if !strings.Contains(body, "# HELP process_cpu_seconds_total") {
		t.Errorf("Want response body to contains '# HELP process_cpu_seconds_total'. But found none in:\n%s", body)
	}

	if !strings.Contains(body, "# HELP go_gc_duration_seconds") {
		t.Errorf("Want response body to contains '# HELP go_gc_duration_seconds'. But found none in:\n%s", body)
	}

	app.sigs <- os.Interrupt
}

func TestCustomMetricsEndpoint(t *testing.T) {

	l := newListenAddrListener()
	defer l.close()

	app := newAppCtx([]string{"--web.listen-address=:0", "--web.telemetry-path=/telemetry", configArg}, os.Stdout, os.Stderr, l)

	go app.run()

	resp, err := http.Get("http://" + l.addr() + "/telemetry")
	if err != nil {
		t.Error(err)
	}

	defer resp.Body.Close()

	statusCode := 200
	if resp.StatusCode != statusCode {
		t.Log("expected:", statusCode)
		t.Log("     got:", resp.StatusCode)
		t.Error("HTTP status code does not match.")
	}

	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Error(err)
	}

	if !strings.HasPrefix(string(responseBody), "# HELP") {
		t.Errorf("Want response body to have prefix '# HELP'. But found none in:\n%s", string(responseBody))
	}

	app.sigs <- os.Interrupt
}

func TestLandingPageDefaultMetricsEndpoint(t *testing.T) {

	l := newListenAddrListener()
	defer l.close()

	app := newAppCtx([]string{"--web.listen-address=:0", configArg}, os.Stdout, os.Stderr, l)

	go app.run()

	resp, err := http.Get("http://" + l.addr() + "/")
	if err != nil {
		t.Error(err)
	}

	defer resp.Body.Close()

	statusCode := 200
	if resp.StatusCode != statusCode {
		t.Log("expected:", statusCode)
		t.Log("     got:", resp.StatusCode)
		t.Error("HTTP status code does not match.")
	}

	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Error(err)
	}

	if !strings.Contains(string(responseBody), "<a href='/metrics'>Metrics</a>") {
		t.Errorf("Want link to default metrics endpoint. But found none in:\n%s", string(responseBody))
	}

	app.sigs <- os.Interrupt
}

func TestLandingPageCustomMetricsEndpoint(t *testing.T) {

	l := newListenAddrListener()
	defer l.close()

	app := newAppCtx([]string{"--web.listen-address=:0", "--web.telemetry-path=/telemetry", configArg}, os.Stdout, os.Stderr, l)

	go app.run()

	resp, err := http.Get("http://" + l.addr() + "/")
	if err != nil {
		t.Error(err)
	}

	defer resp.Body.Close()

	statusCode := 200
	if resp.StatusCode != statusCode {
		t.Log("expected:", statusCode)
		t.Log("     got:", resp.StatusCode)
		t.Error("HTTP status code does not match.")
	}

	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Error(err)
	}

	if !strings.Contains(string(responseBody), "<a href='/telemetry'>Metrics</a>") {
		t.Errorf("Want link to custom metrics endpoint. But found none in:\n%s", string(responseBody))
	}

	app.sigs <- os.Interrupt
}
