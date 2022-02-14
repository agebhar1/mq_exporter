# Copyright 2021-2022 Andreas Gebhardt
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

SOURCE := $(shell find -type f -name "*.go")
FIXTURES := $(find -name "*fixture*" -type d -exec find {} -type f \;)

export OARCH=amd64
export GOOS=linux

.PHONY: all
all: mq_exporter

MQ_LIB := /opt/mqm
GO_GET := .go.get.sentinel
GO_TEST := .go.test.sentinel

$(GO_GET): $(MQ_LIB) go.mod go.sum
	go get
	touch $@

$(GO_TEST): $(GO_GET) $(MQ_LIB) $(SOURCE)
	go test -v ./...
	touch $@

mq_exporter: $(GO_GET) $(GO_TEST) $(MQ_LIB) $(SOURCE)
	go build

.PHONY: clean
clean:
	-rm .go.*.sentinel
	go clean
