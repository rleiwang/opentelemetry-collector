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
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/valyala/gozstd"
	"go.uber.org/zap"

	agenttracepb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/trace/v1"
	"github.com/open-telemetry/opentelemetry-collector/config/configmodels"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	"github.com/open-telemetry/opentelemetry-collector/exporter"
	"github.com/open-telemetry/opentelemetry-collector/exporter/exporterhelper"
)

// New returns a new relay exporter send trace data over HTTP.
func NewRelay(config configmodels.Exporter, logger *zap.Logger, cfg *Config) (exporter.TraceExporter, error) {
	s := &relayHTTPSender{
		url:     cfg.URL,
		headers: cfg.Headers,
		client:  &http.Client{},
		logger:  logger,
		isJSON:  strings.Compare(cfg.Format, formatJSON) == 0,
		ch:      make(chan []byte, 8),
	}

	var ticker *time.Ticker
	if cfg.Interval > 0 {
		logger.Info("start batching relay")
		ticker = time.NewTicker(cfg.Interval * time.Second)
		go s.startBatchRelay(ticker)
	} else {
		logger.Info("start no delay relay", zap.Bool("is", s.isJSON))
		ticker = time.NewTicker(time.Duration(math.MaxInt64))
		go s.startNoDelayRelay(ticker)
	}

	shutdown := func() error {
		if ticker != nil {
			ticker.Stop()
		}
		return nil
	}

	return exporterhelper.NewTraceExporter(
		config,
		s.pushTraceData,
		exporterhelper.WithTracing(true),
		exporterhelper.WithMetrics(true),
		exporterhelper.WithShutdown(shutdown))
}

type relayHTTPSender struct {
	url     string
	headers map[string]string
	client  *http.Client
	logger  *zap.Logger
	isJSON  bool
	ch      chan []byte
}

func (s *relayHTTPSender) pushTraceData(
	ctx context.Context,
	td consumerdata.TraceData,
) (droppedSpans int, err error) {
	atd := &agenttracepb.ExportTraceServiceRequest{
		Spans:    td.Spans,
		Resource: td.Resource,
		Node:     td.Node,
	}

	if s.isJSON {
		jsm := jsonpb.Marshaler{}
		var buf bytes.Buffer
		err = jsm.Marshal(&buf, atd)
		s.ch <- buf.Bytes()
	} else {
		apb, err := proto.Marshal(atd)
		if err != nil {
			return 0, err
		}

		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(buf, uint32(len(apb)))

		s.ch <- append(buf, apb...)
	}

	return 0, nil
}

func (s *relayHTTPSender) startBatchRelay(ticker *time.Ticker) {
	pr, pw := io.Pipe()

	// pipe reader channel
	prc := make(chan *io.PipeReader, 64)
	go s.waitAndSend(prc)

	prc <- pr

loop:
	for {
		select {
		case b, ok := <-s.ch:
			if !ok {
				continue
			}
			pw.Write(b)
		case _, ok := <-ticker.C:
			if !ok {
				break loop
			}
			npr, npw := io.Pipe()
			prc <- npr
			// swap pw and close the old pw
			pw, npw = npw, pw
			npw.Close()
		}
	}
	close(prc)
}

func (s *relayHTTPSender) waitAndSend(cpr chan *io.PipeReader) {
	for r := range cpr {
		s.send(r)
	}
}

func (s *relayHTTPSender) send(pr *io.PipeReader) {
	f, err := ioutil.TempFile(os.TempDir(), "pipe")
	if err != nil {
		return
	}
	defer func() {
		pr.Close()
		f.Close()
		os.Remove(filepath.Join(os.TempDir(), f.Name()))
	}()

	// blocks until pr closed
	err = gozstd.StreamCompress(f, pr)
	if err != nil {
		return
	}

	// jump back to beginning
	f.Seek(0, io.SeekStart)
	req, err := http.NewRequest("POST", s.url, f)
	if err != nil {
		s.logger.Error("create http request", zap.String("url", s.url), zap.Error(err))
		return
	}

	req.Header.Set("Content-Type", "application/zstd")
	if s.headers != nil {
		for k, v := range s.headers {
			req.Header.Set(k, v)
		}
	}

	resp, err := s.client.Do(req)
	if err != nil {
		s.logger.Error("invoke http request", zap.String("url", s.url), zap.Error(err))
		return
	}

	resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		s.logger.Error("http response err", zap.Int("status", resp.StatusCode))
	}
}

func (s *relayHTTPSender) sendJSON(r io.Reader) {
	req, err := http.NewRequest("POST", s.url, r)
	if err != nil {
		s.logger.Error("create http request", zap.String("url", s.url), zap.Error(err))
		return
	}

	req.Header.Set("Content-Type", "application/json")
	if s.headers != nil {
		for k, v := range s.headers {
			req.Header.Set(k, v)
		}
	}

	resp, err := s.client.Do(req)
	if err != nil {
		s.logger.Error("invoke http request", zap.String("url", s.url), zap.Error(err))
		return
	}

	resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		s.logger.Error("http response err", zap.Int("status", resp.StatusCode))
	}
}

func (s *relayHTTPSender) startNoDelayRelay(ticker *time.Ticker) {
loop:
	for {
		select {
		case b, ok := <-s.ch:
			if !ok {
				continue
			}
			s.sendJSON(bytes.NewReader(b))
		case <-ticker.C:
			break loop
		}
	}
}
