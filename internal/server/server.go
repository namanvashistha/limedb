package server

import (
	"context"
	"encoding/json"
	"fmt"
	"limedb/internal/gossiper"
	"limedb/internal/logger"
	"limedb/internal/membership"
	"limedb/internal/messenger"
	"limedb/internal/node"
	"limedb/internal/placement"
	"runtime"
	"sync/atomic"
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
	membership *membership.Manager
	placement  *placement.Manager
	gossiper   *gossiper.Gossiper
	port       int
	httpServer *fasthttp.Server
	tracer     trace.Tracer
	meter      metric.Meter
	reqCounter metric.Int64Counter
	reqLatency metric.Float64Histogram
	startTime  time.Time

	totalRequests  uint64
	totalGets      uint64
	totalSets      uint64
	totalDels      uint64
	totalErrors    uint64
	totalLatencyMs uint64
}

type activateNodeRequest struct {
	NodeURL string `json:"nodeUrl"`
}

// NewServer creates a new HTTP server.
func NewServer(service *node.NodeService, manager *membership.Manager, placementManager *placement.Manager, gossiper *gossiper.Gossiper, port int) *Server {
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
		service:    service,
		membership: manager,
		placement:  placementManager,
		port:       port,
		httpServer: &fasthttp.Server{
			Handler: nil, // Will be set in Start or here
		},
		tracer:     otel.Tracer("limedb-node"),
		meter:      meter,
		reqCounter: reqCounter,
		reqLatency: reqLatency,
		gossiper:   gossiper,
		startTime:  time.Now(),
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

		// Record metrics (use microseconds for better precision)
		duration := float64(time.Since(startTime).Microseconds()) / 1000.0
		status := ctx.Response.StatusCode()

		attrs := metric.WithAttributes(
			attribute.String("http.method", method),
			attribute.String("http.route", path),
			attribute.Int("http.status_code", status),
			attribute.String("node.url", s.service.GetNodeUrl()),
		)

		// Record manual metrics for the health endpoint
		atomic.AddUint64(&s.totalRequests, 1)
		atomic.AddUint64(&s.totalLatencyMs, uint64(duration))

		route := string(ctx.Path())
		if route == "/api/v1/get" {
			atomic.AddUint64(&s.totalGets, 1)
		} else if route == "/api/v1/set" {
			atomic.AddUint64(&s.totalSets, 1)
		} else if route == "/api/v1/del" {
			atomic.AddUint64(&s.totalDels, 1)
		}

		if status >= 400 {
			atomic.AddUint64(&s.totalErrors, 1)
		}

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
	case method == "GET" && path == "/api/v1/keys":
		s.handleListKeys(ctx)
	case method == "GET" && path == "/api/v1/cluster/state":
		s.handleClusterState(ctx)
	case method == "GET" && path == "/api/v1/cluster/ring":
		s.handleRingState(ctx)
	case method == "GET" && path == "/api/v1/cluster/gossip":
		s.handleGossipMetrics(ctx)
	case method == "GET" && path == "/api/v1/admin/membership":
		s.handleMembershipState(ctx)
	case method == "POST" && path == "/api/v1/admin/membership/activate":
		s.handleActivateMembership(ctx)
	case method == "GET" && path == "/api/v1/admin/placement":
		s.handlePlacementState(ctx)
	case method == "POST" && path == "/api/v1/admin/placement/promote":
		s.handlePromotePlacement(ctx)
	case method == "GET" && path == "/api/v1/admin/bootstrap":
		s.handleBootstrapState(ctx)
	case method == "POST" && path == "/api/v1/admin/bootstrap/start":
		s.handleBootstrapStart(ctx)
	case method == "POST" && path == "/api/v1/admin/bootstrap/complete":
		s.handleBootstrapComplete(ctx)
	case method == "GET" && path == "/api/v1/health":
		s.handleHealth(ctx)
	case method == "POST" && path == "/gossip":
		s.handleGossip(ctx)
	case method == "GET" && len(path) > 17 && path[:17] == "/api/v1/replicas/":
		s.handleReplicaInfo(ctx, path[17:])
	// Internal node-to-node replication endpoints (local write only, no QUORUM re-replication)
	case method == "POST" && path == "/internal/replicate":
		s.handleInternalSet(ctx)
	case method == "DELETE" && len(path) > 20 && path[:20] == "/internal/replicate/":
		s.handleInternalDelete(ctx, path[20:])
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

func (s *Server) handleReplicaInfo(ctx *fasthttp.RequestCtx, key string) {
	info := s.service.GetReplicaInfo(key)
	body, _ := json.Marshal(info)
	ctx.SetContentType("application/json")
	ctx.SetBody(body)
}

// handleInternalSet is called by replica coordinators to write locally without triggering QUORUM.
func (s *Server) handleInternalSet(ctx *fasthttp.RequestCtx) {
	if !isInternalRequest(ctx) {
		ctx.Error("Forbidden", fasthttp.StatusForbidden)
		return
	}
	var req node.SetRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		ctx.Error("Invalid JSON", fasthttp.StatusBadRequest)
		return
	}
	s.service.HandleReplicaWrite(req.Key, req.Value)
	ctx.SetBodyString("OK")
}

// handleInternalDelete is called by replica coordinators to delete locally without triggering QUORUM.
func (s *Server) handleInternalDelete(ctx *fasthttp.RequestCtx, key string) {
	if !isInternalRequest(ctx) {
		ctx.Error("Forbidden", fasthttp.StatusForbidden)
		return
	}
	deleted := s.service.HandleReplicaDelete(key)
	if deleted {
		ctx.SetBodyString("1")
	} else {
		ctx.SetBodyString("0")
	}
}

// isInternalRequest returns true if the request originates from a private/loopback IP.
// /internal/* endpoints are only for node-to-node replication — not for external clients.
func isInternalRequest(ctx *fasthttp.RequestCtx) bool {
	ip := ctx.RemoteIP()
	if ip.IsLoopback() {
		return true
	}
	// Allow RFC-1918 private ranges (LAN cluster peers)
	private := []string{"10.", "172.", "192.168."}
	ipStr := ip.String()
	for _, prefix := range private {
		if len(ipStr) >= len(prefix) && ipStr[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

func (s *Server) handleClusterState(ctx *fasthttp.RequestCtx) {
	membershipState := s.membership.GetState()
	state := map[string]interface{}{
		"nodeUrl":         s.service.GetNodeUrl(),
		"peers":           s.membership.GetPeers(),
		"totalNodes":      len(s.membership.GetPeers()),
		"status":          "active",
		"membershipEpoch": membershipState.MembershipEpoch,
		"placementEpoch": func() int64 {
			snapshot := s.placement.Snapshot()
			if snapshot == nil {
				return 0
			}
			return snapshot.Epoch
		}(),
		"pendingPlacementEpoch": func() int64 {
			state := s.placement.State()
			if state == nil || state.Pending == nil {
				return 0
			}
			return state.Pending.Epoch
		}(),
		"activeNodes":   membershipState.ActiveNodes,
		"observedNodes": membershipState.ObservedNodes,
		"desiredNodes":  membershipState.DesiredNodes,
	}

	body, _ := json.Marshal(state)
	ctx.SetContentType("application/json")
	ctx.SetBody(body)
}

func (s *Server) handleMembershipState(ctx *fasthttp.RequestCtx) {
	if !isInternalRequest(ctx) {
		ctx.Error("Forbidden", fasthttp.StatusForbidden)
		return
	}

	body, _ := json.Marshal(s.membership.GetState())
	ctx.SetContentType("application/json")
	ctx.SetBody(body)
}

func (s *Server) handlePlacementState(ctx *fasthttp.RequestCtx) {
	if !isInternalRequest(ctx) {
		ctx.Error("Forbidden", fasthttp.StatusForbidden)
		return
	}

	body, _ := json.Marshal(s.placement.State())
	ctx.SetContentType("application/json")
	ctx.SetBody(body)
}

func (s *Server) handleActivateMembership(ctx *fasthttp.RequestCtx) {
	if !isInternalRequest(ctx) {
		ctx.Error("Forbidden", fasthttp.StatusForbidden)
		return
	}

	var req activateNodeRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		ctx.Error("Invalid JSON", fasthttp.StatusBadRequest)
		return
	}

	if err := s.membership.ActivateNode(req.NodeURL); err != nil {
		ctx.Error(err.Error(), fasthttp.StatusBadRequest)
		return
	}
	pending, err := s.placement.CreatePendingFromMembership(req.NodeURL)
	if err != nil {
		ctx.Error(err.Error(), fasthttp.StatusBadRequest)
		return
	}

	body, _ := json.Marshal(map[string]interface{}{
		"status":           "activation_requested",
		"nodeUrl":          req.NodeURL,
		"membership":       s.membership.GetState(),
		"activePlacement":  s.placement.Snapshot(),
		"pendingPlacement": pending,
	})
	ctx.SetContentType("application/json")
	ctx.SetBody(body)
}

func (s *Server) handlePromotePlacement(ctx *fasthttp.RequestCtx) {
	if !isInternalRequest(ctx) {
		ctx.Error("Forbidden", fasthttp.StatusForbidden)
		return
	}

	active, err := s.placement.PromotePending()
	if err != nil {
		ctx.Error(err.Error(), fasthttp.StatusBadRequest)
		return
	}

	body, _ := json.Marshal(map[string]interface{}{
		"status":          "placement_promoted",
		"activePlacement": active,
	})
	ctx.SetContentType("application/json")
	ctx.SetBody(body)
}

func (s *Server) handleBootstrapState(ctx *fasthttp.RequestCtx) {
	if !isInternalRequest(ctx) {
		ctx.Error("Forbidden", fasthttp.StatusForbidden)
		return
	}

	body, _ := json.Marshal(s.placement.BootstrapPlan())
	ctx.SetContentType("application/json")
	ctx.SetBody(body)
}

func (s *Server) handleBootstrapStart(ctx *fasthttp.RequestCtx) {
	if !isInternalRequest(ctx) {
		ctx.Error("Forbidden", fasthttp.StatusForbidden)
		return
	}

	plan, err := s.placement.StartBootstrap()
	if err != nil {
		ctx.Error(err.Error(), fasthttp.StatusBadRequest)
		return
	}

	body, _ := json.Marshal(map[string]interface{}{
		"status":    "bootstrap_started",
		"bootstrap": plan,
	})
	ctx.SetContentType("application/json")
	ctx.SetBody(body)
}

func (s *Server) handleBootstrapComplete(ctx *fasthttp.RequestCtx) {
	if !isInternalRequest(ctx) {
		ctx.Error("Forbidden", fasthttp.StatusForbidden)
		return
	}

	plan, err := s.placement.CompleteBootstrap()
	if err != nil {
		ctx.Error(err.Error(), fasthttp.StatusBadRequest)
		return
	}

	body, _ := json.Marshal(map[string]interface{}{
		"status":    "bootstrap_completed",
		"bootstrap": plan,
	})
	ctx.SetContentType("application/json")
	ctx.SetBody(body)
}

func (s *Server) handleRingState(ctx *fasthttp.RequestCtx) {
	ring := s.membership.GetRing()
	stats := ring.GetRingStats()
	stats["currentNode"] = s.service.GetCurrentNodeUrl()
	stats["allNodes"] = ring.GetNodes()
	stats["ranges"] = ring.GetNodeRanges()
	stats["rangesDegrees"] = ring.GetNodeRangesDegrees()
	if snapshot := s.placement.Snapshot(); snapshot != nil {
		stats["placementEpoch"] = snapshot.Epoch
	}
	if state := s.placement.State(); state != nil && state.Pending != nil {
		stats["pendingPlacementEpoch"] = state.Pending.Epoch
	}

	body, _ := json.Marshal(stats)
	ctx.SetContentType("application/json")
	ctx.SetBody(body)
}

func (s *Server) handleHealth(ctx *fasthttp.RequestCtx) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Calculate load metrics
	reqs := atomic.LoadUint64(&s.totalRequests)
	gets := atomic.LoadUint64(&s.totalGets)
	sets := atomic.LoadUint64(&s.totalSets)
	dels := atomic.LoadUint64(&s.totalDels)
	errs := atomic.LoadUint64(&s.totalErrors)
	lat := atomic.LoadUint64(&s.totalLatencyMs)

	uptimeSecs := time.Since(s.startTime).Seconds()

	rps := 0.0
	gps := 0.0
	sps := 0.0
	dps := 0.0
	if uptimeSecs > 0 {
		rps = float64(reqs) / uptimeSecs
		gps = float64(gets) / uptimeSecs
		sps = float64(sets) / uptimeSecs
		dps = float64(dels) / uptimeSecs
	}

	avgLat := 0.0
	if reqs > 0 {
		avgLat = float64(lat) / float64(reqs)
	}

	errRate := 0.0
	if reqs > 0 {
		errRate = float64(errs) / float64(reqs)
	}

	health := map[string]interface{}{
		"status":              "healthy",
		"service":             "limedb-node",
		"version":             "1.0.0",
		"nodeUrl":             s.service.GetNodeUrl(),
		"timestamp":           time.Now().Format(time.RFC3339),
		"storage":             s.service.GetStore().Stats(),
		"uptime_seconds":      uptimeSecs,
		"memory_allocated_mb": float64(memStats.Alloc) / 1024 / 1024,
		"memory_sys_mb":       float64(memStats.Sys) / 1024 / 1024,
		"gc_pause_ms":         float64(memStats.PauseTotalNs) / 1e6, // Total GC pause since start
		"goroutines_count":    runtime.NumGoroutine(),
		"requests_per_second": rps,
		"gets_per_second":     gps,
		"sets_per_second":     sps,
		"dels_per_second":     dps,
		"average_latency_ms":  avgLat,
		"error_rate":          errRate,
		"total_requests":      reqs,
	}

	body, _ := json.Marshal(health)
	ctx.SetContentType("application/json")
	ctx.SetBody(body)
}

func (s *Server) handleGossip(ctx *fasthttp.RequestCtx) {
	if s.gossiper == nil {
		logger.Error("Gossiper not initialized")
		ctx.Error("Gossip service unavailable", fasthttp.StatusServiceUnavailable)
		return
	}

	// The request body contains a Message struct from messenger
	requestBody := ctx.PostBody()

	// Extract the GossipMessage from the Message wrapper
	var msg messenger.Message
	if err := json.Unmarshal(requestBody, &msg); err != nil {
		logger.Error("Failed to unmarshal message", "error", err.Error())
		ctx.Error("Invalid message format", fasthttp.StatusBadRequest)
		return
	}

	// Pass the payload (which is the GossipMessage) to the gossiper
	resp := s.gossiper.HandleGossip(msg.Payload)
	body, _ := json.Marshal(resp)
	ctx.SetContentType("application/json")
	ctx.SetBody(body)
}

func (s *Server) handleGossipMetrics(ctx *fasthttp.RequestCtx) {
	if s.gossiper == nil {
		ctx.Error("Gossip service unavailable", fasthttp.StatusServiceUnavailable)
		return
	}

	metrics := s.gossiper.GetGossipMetrics()
	body, _ := json.Marshal(metrics)
	ctx.SetContentType("application/json")
	ctx.SetBody(body)
}

func (s *Server) handleListKeys(ctx *fasthttp.RequestCtx) {
	// Get query parameters for pagination
	args := ctx.QueryArgs()
	page := args.GetUintOrZero("page")
	if page == 0 {
		page = 1
	}
	pageSize := args.GetUintOrZero("pageSize")
	if pageSize == 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100 // Max page size
	}

	// Get all keys from local store (sorted)
	store := s.service.GetStore()
	allKeys := store.ListKeys()
	totalKeys := len(allKeys)

	// Calculate pagination
	startIndex := int((page - 1) * pageSize)
	endIndex := int(startIndex) + int(pageSize)
	if startIndex >= totalKeys {
		startIndex = 0
		endIndex = 0
	}
	if endIndex > totalKeys {
		endIndex = totalKeys
	}

	// Get paginated keys
	paginatedKeys := allKeys[startIndex:endIndex]

	// Build response with key details
	type KeyInfo struct {
		Key   string `json:"key"`
		Value string `json:"value"`
		Size  int    `json:"size"`
	}

	keyInfos := make([]KeyInfo, 0, len(paginatedKeys))
	for _, key := range paginatedKeys {
		value, ok := store.Get(key)
		if ok {
			keyInfos = append(keyInfos, KeyInfo{
				Key:   key,
				Value: value,
				Size:  len(value),
			})
		}
	}

	response := map[string]interface{}{
		"keys":       keyInfos,
		"total":      totalKeys,
		"page":       page,
		"pageSize":   pageSize,
		"totalPages": (totalKeys + int(pageSize) - 1) / int(pageSize),
	}

	body, _ := json.Marshal(response)
	ctx.SetContentType("application/json")
	ctx.SetBody(body)
}
