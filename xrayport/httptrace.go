// Copyright 2017-2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not use this file except in compliance with the License. A copy of the License is located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

// +build go1.8

package xrayport

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http/httptrace"
	"sync"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"
)

// HTTPSpans is a set of context in different HTTP operation.
type HTTPSpans struct {
	opCtx       context.Context
	connCtx     context.Context
	dnsCtx      context.Context
	connectCtx  context.Context
	tlsCtx      context.Context
	reqCtx      context.Context
	responseCtx context.Context
	mu          sync.Mutex
}

// NewHTTPSpans creates a new HTTPSubsegments to use in
// httptrace.ClientTrace functions
func NewHTTPSpans(opCtx context.Context) *HTTPSpans {
	return &HTTPSpans{opCtx: opCtx}
}

// GetConn begins a connect subsegment if the HTTP operation
// subsegment is still in progress.
func (xt *HTTPSpans) GetConn(hostPort string) {
	_, xt.connCtx = opentracing.StartSpanFromContext(xt.opCtx, "connect")
	// if GetSegment(xt.opCtx).safeInProgress() {
	// 	xt.connCtx, _ = BeginSubsegment(xt.opCtx, "connect")
	// }
}

// DNSStart begins a dns subsegment if the HTTP operation
// subsegment is still in progress.
func (xt *HTTPSpans) DNSStart(info httptrace.DNSStartInfo) {
	xt.mu.Lock()
	defer xt.mu.Unlock()
	_, xt.dnsCtx = opentracing.StartSpanFromContext(xt.connCtx, "dns")

	// if GetSegment(xt.opCtx).safeInProgress() && xt.connCtx != nil {
	// 	xt.dnsCtx, _ = BeginSubsegment(xt.connCtx, "dns")
	// }
}

// DNSDone closes the dns subsegment if the HTTP operation
// subsegment is still in progress, passing the error value
// (if any). Information about the address values looked up,
// and whether or not the call was coalesced is added as
// metadata to the dns subsegment.
func (xt *HTTPSpans) DNSDone(info httptrace.DNSDoneInfo) {
	if xt.dnsCtx != nil {
		span := opentracing.SpanFromContext(xt.dnsCtx)
		// span.SetTag("addresses", info.Addrs)
		span.SetTag("coalesced", info.Coalesced)
		if info.Err != nil {
			span.SetTag("error", true)
			span.LogFields(log.String("errors", info.Err.Error()))
		}
		span.Finish()
	}

	// if xt.dnsCtx != nil && GetSegment(xt.opCtx).safeInProgress() {
	// 	metadata := make(map[string]interface{})
	// 	metadata["addresses"] = info.Addrs
	// 	metadata["coalesced"] = info.Coalesced

	// 	AddMetadataToNamespace(xt.dnsCtx, "http", "dns", metadata)
	// 	GetSegment(xt.dnsCtx).Close(info.Err)
	// }
}

// ConnectStart begins a dial subsegment if the HTTP operation
// subsegment is still in progress.
func (xt *HTTPSpans) ConnectStart(network, addr string) {
	xt.mu.Lock()
	defer xt.mu.Unlock()

	if xt.connCtx != nil {
		_, xt.connectCtx = opentracing.StartSpanFromContext(xt.connCtx, "dial")
	}

	// if GetSegment(xt.opCtx).safeInProgress() && xt.connCtx != nil {
	// 	xt.connectCtx, _ = BeginSubsegment(xt.connCtx, "dial")
	// }
}

// ConnectDone closes the dial subsegment if the HTTP operation
// subsegment is still in progress, passing the error value
// (if any). Information about the network over which the dial
// was made is added as metadata to the subsegment.
func (xt *HTTPSpans) ConnectDone(network, addr string, err error) {
	if xt.connectCtx != nil {
		span := opentracing.SpanFromContext(xt.connectCtx)
		span.SetTag("network", network)

		if err != nil {
			span.SetTag("error", true)
			span.LogFields(log.String("errors", err.Error()))
		}

		span.Finish()
	}

	// if xt.connectCtx != nil && GetSegment(xt.opCtx).safeInProgress() {
	// 	metadata := make(map[string]interface{})
	// 	metadata["network"] = network

	// 	AddMetadataToNamespace(xt.connectCtx, "http", "connect", metadata)
	// 	GetSegment(xt.connectCtx).Close(err)
	// }
}

// TLSHandshakeStart begins a tls subsegment if the HTTP operation
// subsegment is still in progress.
func (xt *HTTPSpans) TLSHandshakeStart() {
	if xt.connCtx != nil {
		_, xt.tlsCtx = opentracing.StartSpanFromContext(xt.connCtx, "tls")
	}

	// if GetSegment(xt.opCtx).safeInProgress() && xt.connCtx != nil {
	// 	xt.tlsCtx, _ = BeginSubsegment(xt.connCtx, "tls")
	// }
}

// TLSHandshakeDone closes the tls subsegment if the HTTP
// operation subsegment is still in progress, passing the
// error value(if any). Information about the tls connection
// is added as metadata to the subsegment.
func (xt *HTTPSpans) TLSHandshakeDone(connState tls.ConnectionState, err error) {
	if xt.tlsCtx != nil {
		span := opentracing.SpanFromContext(xt.tlsCtx)

		span.SetTag("did_resume", connState.DidResume)
		span.SetTag("negotiated_protocol", connState.NegotiatedProtocol)
		span.SetTag("negotiated_protocol_is_mutual", connState.NegotiatedProtocolIsMutual)
		span.SetTag("cipher_suite", connState.CipherSuite)

		if err != nil {
			span.SetTag("error", true)
			span.LogFields(log.String("errors", err.Error()))
		}

		span.Finish()
	}

	// if xt.tlsCtx != nil && GetSegment(xt.opCtx).safeInProgress() {
	// 	metadata := make(map[string]interface{})
	// 	metadata["did_resume"] = connState.DidResume
	// 	metadata["negotiated_protocol"] = connState.NegotiatedProtocol
	// 	metadata["negotiated_protocol_is_mutual"] = connState.NegotiatedProtocolIsMutual
	// 	metadata["cipher_suite"] = connState.CipherSuite

	// 	AddMetadataToNamespace(xt.tlsCtx, "http", "tls", metadata)
	// 	GetSegment(xt.tlsCtx).Close(err)
	// }
}

// GotConn closes the connect subsegment if the HTTP operation
// subsegment is still in progress, passing the error value
// (if any). Information about the connection is added as
// metadata to the subsegment. If the connection is marked as reused,
// the connect subsegment is deleted.
func (xt *HTTPSpans) GotConn(info *httptrace.GotConnInfo, err error) {
	if xt.connCtx != nil {
		span := opentracing.SpanFromContext(xt.connCtx)

		if info != nil { // XXX Why only checked here?
			span.SetTag("reused", info.Reused)
			span.SetTag("was_idle", info.WasIdle)
			if info.WasIdle {
				span.SetTag("idle_time", info.IdleTime)
			}
		}

		if err != nil {
			span.SetTag("error", true)
			span.LogFields(log.String("errors", err.Error()))
		} else {
			_, xt.reqCtx = opentracing.StartSpanFromContext(xt.opCtx, "request")
		}

		span.Finish()
	}

	// if xt.connCtx != nil && GetSegment(xt.opCtx).safeInProgress() { // GetConn may not have been called (client_test.TestBadRoundTrip)
	// 	if info != nil {
	// 		if info.Reused {
	// 			GetSegment(xt.opCtx).RemoveSubsegment(GetSegment(xt.connCtx))
	// 			xt.mu.Lock()
	// 			// Remove the connCtx context since it is no longer needed.
	// 			xt.connCtx = nil
	// 			xt.mu.Unlock()
	// 		} else {
	// 			metadata := make(map[string]interface{})
	// 			metadata["reused"] = info.Reused
	// 			metadata["was_idle"] = info.WasIdle
	// 			if info.WasIdle {
	// 				metadata["idle_time"] = info.IdleTime
	// 			}

	// 			AddMetadataToNamespace(xt.connCtx, "http", "connection", metadata)
	// 			GetSegment(xt.connCtx).Close(err)
	// 		}
	// 	} else if xt.connCtx != nil && GetSegment(xt.connCtx).safeInProgress() {
	// 		GetSegment(xt.connCtx).Close(err)
	// 	}

	// 	if err == nil {
	// 		xt.reqCtx, _ = BeginSubsegment(xt.opCtx, "request")
	// 	}

	// }
}

// WroteRequest closes the request subsegment if the HTTP operation
// subsegment is still in progress, passing the error value
// (if any). The response subsegment is then begun.
func (xt *HTTPSpans) WroteRequest(info httptrace.WroteRequestInfo) {
	if xt.reqCtx != nil {
		span := opentracing.SpanFromContext(xt.reqCtx)

		if info.Err != nil {
			span.SetTag("error", true)
			span.LogFields(log.String("errors", info.Err.Error()))
		}

		span.Finish()

		_, resCtx := opentracing.StartSpanFromContext(xt.opCtx, "response")
		xt.mu.Lock() // XXX Why only here?
		xt.responseCtx = resCtx
		xt.mu.Unlock()
	}

	// if xt.reqCtx != nil && GetSegment(xt.opCtx).InProgress {
	// 	GetSegment(xt.reqCtx).Close(info.Err)
	// 	resCtx, _ := BeginSubsegment(xt.opCtx, "response")
	// 	xt.mu.Lock()
	// 	xt.responseCtx = resCtx
	// 	xt.mu.Unlock()
	// }

	if xt.connCtx != nil {
		span := opentracing.SpanFromContext(xt.connCtx)
		span.Finish()
	}

	// In case the GotConn http trace handler wasn't called,
	// we close the connection subsegment since a connection
	// had to have been acquired before attempting to write
	// the request.
	// if xt.connCtx != nil && GetSegment(xt.connCtx).safeInProgress() {
	// 	GetSegment(xt.connCtx).Close(nil)
	// }
}

// GotFirstResponseByte closes the response subsegment if the HTTP
// operation subsegment is still in progress.
func (xt *HTTPSpans) GotFirstResponseByte() {
	xt.mu.Lock()
	resCtx := xt.responseCtx
	xt.mu.Unlock()

	if resCtx != nil {
		span := opentracing.SpanFromContext(resCtx)
		span.Finish()
	}

	// if resCtx != nil && GetSegment(xt.opCtx).InProgress {
	// 	GetSegment(resCtx).Close(nil)
	// }
}

// ClientTrace is a set of pointers of HTTPSubsegments and ClientTrace.
type ClientTrace struct {
	spans     *HTTPSpans
	httpTrace *httptrace.ClientTrace
}

// NewClientTrace returns an instance of xray.ClientTrace, a wrapper
// around httptrace.ClientTrace. The ClientTrace implementation will
// generate subsegments for connection time, DNS lookup time, TLS
// handshake time, and provides additional information about the HTTP round trip
func NewClientTrace(opCtx context.Context) (ct *ClientTrace, err error) {
	if opCtx == nil {
		return nil, errors.New("opCtx must be non-nil")
	}

	spans := NewHTTPSpans(opCtx)

	return &ClientTrace{
		spans: spans,
		httpTrace: &httptrace.ClientTrace{
			GetConn: func(hostPort string) {
				spans.GetConn(hostPort)
			},
			DNSStart: func(info httptrace.DNSStartInfo) {
				spans.DNSStart(info)
			},
			DNSDone: func(info httptrace.DNSDoneInfo) {
				spans.DNSDone(info)
			},
			ConnectStart: func(network, addr string) {
				spans.ConnectStart(network, addr)
			},
			ConnectDone: func(network, addr string, err error) {
				spans.ConnectDone(network, addr, err)
			},
			TLSHandshakeStart: func() {
				spans.TLSHandshakeStart()
			},
			TLSHandshakeDone: func(connState tls.ConnectionState, err error) {
				spans.TLSHandshakeDone(connState, err)
			},
			GotConn: func(info httptrace.GotConnInfo) {
				spans.GotConn(&info, nil)
			},
			WroteRequest: func(info httptrace.WroteRequestInfo) {
				spans.WroteRequest(info)
			},
			GotFirstResponseByte: func() {
				spans.GotFirstResponseByte()
			},
		},
	}, nil
}
