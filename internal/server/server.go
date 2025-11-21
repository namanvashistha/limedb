package server

import (
	"encoding/json"
	"fmt"
	"limedb-go/internal/node"
	"log"

	"github.com/valyala/fasthttp"
)

// Server represents the HTTP server.
type Server struct {
	service    *node.NodeService
	port       int
	httpServer *fasthttp.Server
}

// New creates a new HTTP server.
func New(service *node.NodeService, port int) *Server {
	return &Server{
		service: service,
		port:    port,
		httpServer: &fasthttp.Server{
			Handler: nil, // Will be set in Start or here
		},
	}
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	s.httpServer.Handler = s.router
	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("Starting HTTP server on %s", addr)
	return s.httpServer.ListenAndServe(addr)
}

// Shutdown gracefully shuts down the HTTP server.
func (s *Server) Shutdown() error {
	return s.httpServer.Shutdown()
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
	default:
		ctx.Error("Not Found", fasthttp.StatusNotFound)
	}
}

func (s *Server) handleGet(ctx *fasthttp.RequestCtx, key string) {
	resp, err := s.service.HandleGet(key)
	if err != nil {
		ctx.Error(err.Error(), fasthttp.StatusNotFound)
		return
	}

	body, _ := json.Marshal(resp)
	ctx.SetContentType("application/json")
	ctx.SetBody(body)
}

func (s *Server) handleSet(ctx *fasthttp.RequestCtx) {
	var req node.SetRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		ctx.Error("Invalid JSON", fasthttp.StatusBadRequest)
		return
	}

	if err := s.service.HandleSet(req.Key, req.Value); err != nil {
		ctx.Error(err.Error(), fasthttp.StatusInternalServerError)
		return
	}

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
