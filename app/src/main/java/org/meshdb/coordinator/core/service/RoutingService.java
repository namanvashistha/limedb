package org.meshdb.coordinator.core.service;

import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.ResponseEntity;
import org.springframework.stereotype.Service;
import org.springframework.web.client.RestTemplate;
import org.springframework.web.client.ResourceAccessException;

import java.util.Map;

@Service
public class RoutingService {
    
    @Autowired
    private ShardRegistryService shardRegistry;
    
    @Autowired
    private RestTemplate restTemplate;

    public String get(String key) {
        String shardUrl = shardRegistry.getShardByKey(key);
        try {
            ResponseEntity<String> response = restTemplate.getForEntity(
                shardUrl + "/api/v1/get/" + key, 
                String.class
            );
            return response.getBody();
        } catch (ResourceAccessException e) {
            throw new RuntimeException("Shard " + shardUrl + " is unavailable", e);
        }
    }

    public String set(String key, String value) {
        String shardUrl = shardRegistry.getShardByKey(key);
        try {
            Map<String, String> request = Map.of("key", key, "value", value);
            ResponseEntity<String> response = restTemplate.postForEntity(
                shardUrl + "/api/v1/set",
                request,
                String.class
            );
            return response.getBody();
        } catch (ResourceAccessException e) {
            throw new RuntimeException("Shard " + shardUrl + " is unavailable", e);
        }
    }

    public String delete(String key) {
        String shardUrl = shardRegistry.getShardByKey(key);
        try {
            ResponseEntity<String> response = restTemplate.exchange(
                shardUrl + "/api/v1/del/" + key,
                org.springframework.http.HttpMethod.DELETE,
                null,
                String.class
            );
            return response.getBody();
        } catch (ResourceAccessException e) {
            throw new RuntimeException("Shard " + shardUrl + " is unavailable", e);
        }
    }
}
