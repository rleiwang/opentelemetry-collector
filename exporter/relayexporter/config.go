// Copyright 2019, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package relayexporter

import (
	"time"

	"github.com/open-telemetry/opentelemetry-collector/config/configmodels"
)

const (
	formatProto     = "proto"
	formatJSON      = "json"
	defaultInterval = 60
)

// Config defines configuration for relay exporter.
type Config struct {
	configmodels.ExporterSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.

	// URL is the URL of the relay receiver (e.g.: http://some.url:14268/api/traces).
	URL string `mapstructure:"url"`

	// Interval defines time in seconds bewteen batch uploading tracedata to relay receiver
	// default value is 60 seconds. zero means do not wait.
	Interval time.Duration `mapstructure:"interval"`

	Format string `mapstructure:"format"`

	// Headers are a set of headers to be added to the HTTP request sending relay
	Headers map[string]string `mapstructure:"headers"`
}
