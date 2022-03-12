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

PKG := github.com/prometheus/common/version

VERSION := -X '$(PKG).Version=$(shell git describe --exact-match --abbrev=0 2>/dev/null || echo "n/a")'
BRANCH := -X '$(PKG).Branch=$(shell git rev-parse --abbrev-ref HEAD)'
REVISION := -X '$(PKG).Revision=$(shell git rev-list -1 HEAD)'
BUILD_USER := -X '$(PKG).BuildUser=$(shell whoami)@$(shell cat /etc/hostname)'
BUILD_DATE := -X '$(PKG).BuildDate=$(shell date '+%Y%m%d-%H:%M:%S')'

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
	go build -ldflags="$(VERSION) $(BRANCH) $(REVISION) $(BUILD_USER) $(BUILD_DATE)"

.PHONY: clean
clean:
	-rm .go.*.sentinel
	go clean
