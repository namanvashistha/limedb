package server

import (
	"context"
	"encoding/json"
	"fmt"
	"limedb-go/internal/logger"
	"limedb-go/internal/node"
	"time"

	"github.com/valyala/fasthttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Server represents the HTTP server.
type Server struct {
	service    *node.NodeService
	port       int
	httpServer *fasthttp.Server
	tracer     trace.Tracer
	meter      metric.Meter
	reqCounter metric.Int64Counter
	reqLatency metric.Float64Histogram
}

// New creates a new HTTP server.
func New(service *node.NodeService, port int) *Server {
	meter := otel.Meter("limedb-node")

	reqCounter, _ := meter.Int64Counter(
		"http.server.request_count",
		metric.WithDescription("Total number of HTTP requests"),
		metric.WithUnit("{request}"),
	)

	reqLatency, _ := meter.Float64Histogram(
		"http.server.request_duration",
		metric.WithDescription("Duration of HTTP requests"),
		metric.WithUnit("ms"),
	)

	return &Server{
		service: service,
		port:    port,
		httpServer: &fasthttp.Server{
			Handler: nil, // Will be set in Start or here
		},
		tracer:     otel.Tracer("limedb-node"),
		meter:      meter,
		reqCounter: reqCounter,
		reqLatency: reqLatency,
	}
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	s.httpServer.Handler = s.traceMiddleware(s.router)
	addr := fmt.Sprintf(":%d", s.port)
	logger.Info("Starting HTTP server", "address", addr)
	return s.httpServer.ListenAndServe(addr)
}

// Shutdown gracefully shuts down the HTTP server.
func (s *Server) Shutdown() error {
	return s.httpServer.Shutdown()
}

// traceMiddleware wraps the router to add OTel tracing.
func (s *Server) traceMiddleware(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		// Extract trace context from headers
		propagator := otel.GetTextMapPropagator()
		// Adapter to allow OTel to read headers from fasthttp
		headerCarrier := &fasthttpHeaderCarrier{ctx: ctx}
		parentCtx := propagator.Extract(context.Background(), headerCarrier)

		// Start a new span
		path := string(ctx.Path())
		method := string(ctx.Method())
		spanName := fmt.Sprintf("%s %s", method, path)

		startTime := time.Now()
		tracedCtx, span := s.tracer.Start(parentCtx, spanName)
		defer span.End()

		// Add span attributes including node URL
		span.SetAttributes(
			attribute.String("http.method", method),
			attribute.String("http.route", path),
			attribute.String("node.url", s.service.GetNodeUrl()),
		)

		// Store the traced context in the UserValue so handlers can access it if needed
		// Note: fasthttp doesn't use context.Context natively for cancellation in the same way net/http does,
		// but we need it for OTel propagation.
		ctx.SetUserValue("tracedCtx", tracedCtx)

		next(ctx)

		// Record metrics
		duration := float64(time.Since(startTime).Milliseconds())
		status := ctx.Response.StatusCode()

		attrs := metric.WithAttributes(
			attribute.String("http.method", method),
			attribute.String("http.route", path),
			attribute.Int("http.status_code", status),
			attribute.String("node.url", s.service.GetNodeUrl()),
		)

		s.reqCounter.Add(tracedCtx, 1, attrs)
		s.reqLatency.Record(tracedCtx, duration, attrs)
	}
}

// fasthttpHeaderCarrier adapts fasthttp.RequestCtx to propagation.TextMapCarrier
type fasthttpHeaderCarrier struct {
	ctx *fasthttp.RequestCtx
}

func (c *fasthttpHeaderCarrier) Get(key string) string {
	return string(c.ctx.Request.Header.Peek(key))
}

func (c *fasthttpHeaderCarrier) Set(key, value string) {
	c.ctx.Response.Header.Set(key, value)
}

func (c *fasthttpHeaderCarrier) Keys() []string {
	// Not efficiently supported by fasthttp, but rarely needed for Extract
	return nil
}

// router handles incoming HTTP requests.
func (s *Server) router(ctx *fasthttp.RequestCtx) {
	path := string(ctx.Path())
	method := string(ctx.Method())

	switch {
	case method == "GET" && len(path) > 12 && path[:12] == "/api/v1/get/":
		s.handleGet(ctx, path[12:])
	case method == "POST" && path == "/api/v1/set":
		s.handleSet(ctx)
	case method == "DELETE" && len(path) > 12 && path[:12] == "/api/v1/del/":
		s.handleDelete(ctx, path[12:])
	case method == "GET" && path == "/api/v1/cluster/state":
		s.handleClusterState(ctx)
	case method == "GET" && path == "/api/v1/cluster/ring":
		s.handleRingState(ctx)
	case method == "GET" && path == "/api/v1/health":
		s.handleHealth(ctx)
	default:
		ctx.Error("Not Found", fasthttp.StatusNotFound)
	}
}

func (s *Server) handleGet(ctx *fasthttp.RequestCtx, key string) {
	logger.Info("GET request",
		"key", key,
		"client.ip", ctx.RemoteIP().String(),
	)
	
	resp, err := s.service.HandleGet(key)
	if err != nil {
		logger.Warn("GET failed", "key", key, "error", err.Error())
		ctx.Error(err.Error(), fasthttp.StatusNotFound)
		return
	}

	logger.Info("GET success", "key", key, "node", resp.NodeUrl)
	body, _ := json.Marshal(resp)
	ctx.SetContentType("application/json")
	ctx.SetBody(body)
}

func (s *Server) handleSet(ctx *fasthttp.RequestCtx) {
	var req node.SetRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		logger.Error("SET invalid JSON", "error", err.Error())
		ctx.Error("Invalid JSON", fasthttp.StatusBadRequest)
		return
	}

	logger.Info("SET request", "key", req.Key, "size", len(req.Value))

	if err := s.service.HandleSet(req.Key, req.Value); err != nil {
		logger.Error("SET failed", "key", req.Key, "error", err.Error())
		ctx.Error(err.Error(), fasthttp.StatusInternalServerError)
		return
	}

	logger.Info("SET success", "key", req.Key)
	ctx.SetBodyString("OK")
}

func (s *Server) handleDelete(ctx *fasthttp.RequestCtx, key string) {
	deleted, err := s.service.HandleDelete(key)
	if err != nil {
		ctx.Error(err.Error(), fasthttp.StatusInternalServerError)
		return
	}

	if deleted {
		ctx.SetBodyString("1")
	} else {
		ctx.SetBodyString("0")
	}
}

func (s *Server) handleClusterState(ctx *fasthttp.RequestCtx) {
	state := map[string]interface{}{
		"nodeUrl":    s.service.GetNodeUrl(),
		"peers":      s.service.GetPeers(),
		"totalNodes": len(s.service.GetPeers()),
		"status":     "active",
	}

	body, _ := json.Marshal(state)
	ctx.SetContentType("application/json")
	ctx.SetBody(body)
}

func (s *Server) handleRingState(ctx *fasthttp.RequestCtx) {
	ring := s.service.GetRing()
	stats := ring.GetRingStats()
	stats["currentNode"] = s.service.GetCurrentNodeUrl()
	stats["allNodes"] = ring.GetNodes()
	stats["ranges"] = ring.GetNodeRanges()
	stats["rangesDegrees"] = ring.GetNodeRangesDegrees()

	body, _ := json.Marshal(stats)
	ctx.SetContentType("application/json")
	ctx.SetBody(body)
}

func (s *Server) handleHealth(ctx *fasthttp.RequestCtx) {
	health := map[string]interface{}{
		"status":    "healthy",
		"service":   "limedb-node",  
		"version":   "1.0.0",
		"nodeUrl":   s.service.GetNodeUrl(),
		"timestamp": time.Now().Format(time.RFC3339),
	}

	body, _ := json.Marshal(health)
	ctx.SetContentType("application/json")
	ctx.SetBody(body)
}
