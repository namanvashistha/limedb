package org.meshdb.coordinator.core.controller;

import org.meshdb.coordinator.core.service.RoutingService;
import org.meshdb.coordinator.core.service.ShardRegistryService;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;

import java.util.Map;

@RestController
@ConditionalOnProperty(name = "node.type", havingValue = "coordinator")
@RequestMapping("/api/v1")
public class CoordinatorController {
    
    @Autowired
    private RoutingService routingService;
    
    @Autowired
    private ShardRegistryService shardRegistry;

    @GetMapping("/get/{key}")
    public ResponseEntity<String> get(@PathVariable String key) {
        try {
            String value = routingService.get(key);
            if (value == null || value.isEmpty()) {
                return ResponseEntity.notFound().build();
            }
            return ResponseEntity.ok(value);
        } catch (Exception e) {
            return ResponseEntity.internalServerError().body("Error: " + e.getMessage());
        }
    }

    @PostMapping("/set")
    public ResponseEntity<String> set(@RequestBody Map<String, String> request) {
        try {
            String key = request.get("key");
            String value = request.get("value");
            
            if (key == null || value == null) {
                return ResponseEntity.badRequest().body("Missing key or value");
            }
            
            String result = routingService.set(key, value);
            return ResponseEntity.ok(result);
        } catch (Exception e) {
            return ResponseEntity.internalServerError().body("Error: " + e.getMessage());
        }
    }

    @DeleteMapping("/del/{key}")
    public ResponseEntity<String> delete(@PathVariable String key) {
        try {
            String result = routingService.delete(key);
            return ResponseEntity.ok(result);
        } catch (Exception e) {
            return ResponseEntity.internalServerError().body("Error: " + e.getMessage());
        }
    }

    @GetMapping("/health")
    public ResponseEntity<Map<String, Object>> health() {
        return ResponseEntity.ok(Map.of(
            "status", "healthy",
            "type", "coordinator",
            "shardCount", shardRegistry.getShardCount(),
            "shards", shardRegistry.getAllShards()
        ));
    }
}