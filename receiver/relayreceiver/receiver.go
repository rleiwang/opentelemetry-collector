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

package relayreceiver

import (
	"bytes"
	"encoding/binary"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/golang/protobuf/proto"
	"github.com/valyala/gozstd"

	agenttracepb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/trace/v1"
	"github.com/open-telemetry/opentelemetry-collector/consumer"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	"github.com/open-telemetry/opentelemetry-collector/receiver"
)

const source string = "relay"

// Receiver is the type that exposes Trace and Metrics reception.
type relay struct {
	endpoint    string
	server      *http.Server
	nextTracer  consumer.TraceConsumer
	nextMetrics consumer.MetricsConsumer
}

func New(addr string, tc consumer.TraceConsumer, mc consumer.MetricsConsumer) (*relay, error) {
	return &relay{endpoint: addr, nextTracer: tc, nextMetrics: mc}, nil
}

// TraceSource returns the name of the trace data source.
func (r *relay) TraceSource() string {
	return source
}

// StartTraceReception runs the trace receiver on the gRPC server. Currently
// it also enables the metrics receiver too.
func (r *relay) StartTraceReception(host receiver.Host) error {
	cln, err := net.Listen("tcp", r.endpoint)
	if err != nil {
		return err
	}

	r.server = &http.Server{Handler: r}
	go func() {
		// start server
		r.server.Serve(cln)
	}()

	return nil
}

// StopTraceReception tells the receiver that should stop reception,
// giving it a chance to perform any necessary clean-up.
func (r *relay) StopTraceReception() error {
	return r.server.Close()
}

func (r *relay) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	f, err := ioutil.TempFile(os.TempDir(), "pipe")
	if err != nil {
		return
	}
	defer func() {
		req.Body.Close()
		f.Close()
		os.Remove(filepath.Join(os.TempDir(), f.Name()))
	}()

	err = gozstd.StreamDecompress(f, req.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	f.Seek(0, io.SeekStart)

	buf := bytes.NewBuffer(make([]byte, 0, 1024))
	for {
		buf.Reset()
		_, err = io.CopyN(buf, f, 4)
		if err != nil {
			// read till io.EOF
			break
		}
		n := binary.LittleEndian.Uint32(buf.Bytes())
		buf.Reset()
		_, err = io.CopyN(buf, f, int64(n))
		if err != nil {
			break
		}

		tr := agenttracepb.ExportTraceServiceRequest{}
		proto.Unmarshal(buf.Bytes(), &tr)
		r.nextTracer.ConsumeTraceData(req.Context(), consumerdata.TraceData{
			Node:     tr.GetNode(),
			Resource: tr.GetResource(),
			Spans:    tr.GetSpans(),
		})
	}
}
