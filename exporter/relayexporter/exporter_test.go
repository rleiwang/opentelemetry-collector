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
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector/config/configmodels"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
)

const testHTTPAddress = "http://a.test.dom:123/at/some/path"

type args struct {
	config configmodels.Exporter
	conf   *Config
}

func TestNew(t *testing.T) {
	args := args{
		config: &configmodels.ExporterSettings{},
		conf: &Config{
			URL:      testHTTPAddress,
			Headers:  map[string]string{"test": "test"},
			Interval: 10 * time.Nanosecond,
		},
	}

	got, err := NewRelay(args.config, zap.NewNop(), args.conf)
	assert.NoError(t, err)
	require.NotNil(t, got)

	err = got.ConsumeTraceData(context.Background(), consumerdata.TraceData{})
	assert.NoError(t, err)
}

func TestNewFailsWithEmptyExporterName(t *testing.T) {
	args := args{
		config: nil,
		conf: &Config{
			URL:      testHTTPAddress,
			Headers:  map[string]string{"test": "test"},
			Interval: 10 * time.Nanosecond,
		},
	}

	got, err := NewRelay(args.config, zap.NewNop(), args.conf)
	assert.EqualError(t, err, "nil config")
	assert.Nil(t, got)
}
