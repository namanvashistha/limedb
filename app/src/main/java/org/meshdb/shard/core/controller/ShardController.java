package org.meshdb.shard.core.controller;

import org.meshdb.shard.core.dto.SetRequest;
import org.meshdb.shard.core.service.ShardService;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;

@RestController
@ConditionalOnProperty(name = "node.type", havingValue = "shard")
@RequestMapping("/api/v1")
public class ShardController {
    private final ShardService service;

    public ShardController(ShardService service) {
        this.service = service;
    }

    // GET /get/:key - Get value of a key
    @GetMapping("/get/{key}")
    public ResponseEntity<String> get(@PathVariable String key) {
        String value = service.get(key);
        if (value != null) {
            return ResponseEntity.ok(value);
        } else {
            return ResponseEntity.notFound().build();
        }
    }

    // POST /set - Set value of a key
    @PostMapping("/set")
    public ResponseEntity<String> set(@RequestBody SetRequest request) {
        service.set(request.key(), request.value());
        return ResponseEntity.ok("OK");
    }

    // DELETE /del/:key - Delete a key
    @DeleteMapping("/del/{key}")
    public ResponseEntity<String> delete(@PathVariable String key) {
        boolean deleted = service.delete(key);
        if (deleted) {
            return ResponseEntity.ok("1"); // Redis returns 1 for successful deletion
        } else {
            return ResponseEntity.ok("0"); // Redis returns 0 for key not found
        }
    }

    @ExceptionHandler(Exception.class)
    public ResponseEntity<String> handleException(Exception ex) {
        return ResponseEntity.status(HttpStatus.INTERNAL_SERVER_ERROR).body("Error: " + ex.getMessage());
    }
}