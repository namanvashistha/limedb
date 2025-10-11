package org.meshdb.coordinator.core.service;

import org.springframework.stereotype.Service;
import java.util.List;

@Service
public class ShardRegistryService {
    
    // In-memory list of shard URLs
    private final List<String> shards = List.of(
        "http://localhost:7001",
        "http://localhost:7002",
        "http://localhost:7003"
    );

    public String getShardByKey(String key) {
        if (shards.isEmpty()) {
            throw new RuntimeException("No shards available");
        }
        
        // Hash the key and use modulo to select shard
        int index = Math.abs(key.hashCode()) % shards.size();
        return shards.get(index);
    }

    public List<String> getAllShards() {
        return shards;
    }

    public int getShardCount() {
        return shards.size();
    }
}
