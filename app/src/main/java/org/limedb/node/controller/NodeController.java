package org.limedb.node.controller;

import org.limedb.node.dto.SetRequest;
import org.limedb.node.service.NodeService;
import org.limedb.node.routing.RoutingService;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

import java.util.List;
import java.util.Map;

@RestController
@RequestMapping("/api/v1")
public class NodeController {
    private final NodeService service;
    private final RoutingService routingService;
    
    @Autowired
    private int nodeId;
    
    @Autowired
    private List<String> peerUrls;

    public NodeController(NodeService service, RoutingService routingService) {
        this.service = service;
        this.routingService = routingService;
    }

    // GET /get/:key - Get value of a key (with peer-to-peer routing)
    @GetMapping("/get/{key}")
    public ResponseEntity<String> get(@PathVariable String key) {
        try {
            return service.handleGet(key);
        } catch (Exception e) {
            return ResponseEntity.status(HttpStatus.INTERNAL_SERVER_ERROR)
                    .body("Error: " + e.getMessage());
        }
    }

    // POST /set - Set value of a key (with peer-to-peer routing)
    @PostMapping("/set")
    public ResponseEntity<String> set(@RequestBody SetRequest request) {
        try {
            return service.handleSet(request.key(), request.value());
        } catch (Exception e) {
            return ResponseEntity.status(HttpStatus.INTERNAL_SERVER_ERROR)
                    .body("Error: " + e.getMessage());
        }
    }

    // DELETE /del/:key - Delete a key (with peer-to-peer routing)
    @DeleteMapping("/del/{key}")
    public ResponseEntity<String> delete(@PathVariable String key) {
        try {
            return service.handleDelete(key);
        } catch (Exception e) {
            return ResponseEntity.status(HttpStatus.INTERNAL_SERVER_ERROR)
                    .body("Error: " + e.getMessage());
        }
    }

    // GET /cluster/state - Show cluster state information
    @GetMapping("/cluster/state")
    public ResponseEntity<Map<String, Object>> clusterState() {
        return ResponseEntity.ok(Map.of(
            "nodeId", nodeId,
            "peers", peerUrls,
            "totalNodes", peerUrls.size(),
            "status", "active"
        ));
    }

    // GET /cluster/ring - Show consistent hash ring statistics
    @GetMapping("/cluster/ring")
    public ResponseEntity<Map<String, Object>> ringState() {
        try {
            Map<String, Object> ringStats = routingService.getRingStatistics();
            ringStats.put("currentNode", routingService.getCurrentNodeUrl());
            ringStats.put("allNodes", routingService.getAllNodes());
            return ResponseEntity.ok(ringStats);
        } catch (Exception e) {
            return ResponseEntity.status(HttpStatus.INTERNAL_SERVER_ERROR)
                    .body(Map.of("error", e.getMessage()));
        }
    }

    @ExceptionHandler(Exception.class)
    public ResponseEntity<String> handleException(Exception ex) {
        return ResponseEntity.status(HttpStatus.INTERNAL_SERVER_ERROR).body("Error: " + ex.getMessage());
    }
}