package org.limedb.node.service;

import org.limedb.node.dto.SetRequest;
import org.limedb.node.repository.NodeRepository;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.ResponseEntity;
import org.springframework.stereotype.Service;
import org.springframework.web.client.RestTemplate;
import org.springframework.web.client.ResourceAccessException;

import java.util.List;
import java.util.Optional;

@Service
public class NodeService {

    private final NodeRepository repository;
    
    @Autowired
    private int nodeId;
    
    @Autowired
    private List<String> peerUrls;
    
    @Autowired
    private RestTemplate restTemplate;

    public NodeService(NodeRepository repository) {
        this.repository = repository;
    }

    /**
     * Determine which node should handle the given key (1-based)
     */
    public int getTargetNodeId(String key) {
        if (peerUrls == null || peerUrls.isEmpty()) {
            throw new RuntimeException("No peers configured");
        }
        
        // Use simple modulo-based routing with String.hashCode()
        // Convert from 0-based to 1-based: Node 1, 2, 3...
        return (Math.abs(key.hashCode()) % peerUrls.size()) + 1;
    }

    /**
     * Check if this node should handle the request locally
     */
    public boolean shouldHandleLocally(String key) {
        return getTargetNodeId(key) == nodeId;
    }

    /**
     * Handle GET request - either locally or forward to peer
     */
    public ResponseEntity<String> handleGet(String key) {
        if (shouldHandleLocally(key)) {
            String value = getLocal(key);
            return value != null ? ResponseEntity.ok(value) : ResponseEntity.notFound().build();
        } else {
            return forwardGet(key);
        }
    }

    /**
     * Handle SET request - either locally or forward to peer
     */
    public ResponseEntity<String> handleSet(String key, String value) {
        if (shouldHandleLocally(key)) {
            setLocal(key, value);
            return ResponseEntity.ok("OK");
        } else {
            return forwardSet(key, value);
        }
    }

    /**
     * Handle DELETE request - either locally or forward to peer
     */
    public ResponseEntity<String> handleDelete(String key) {
        if (shouldHandleLocally(key)) {
            boolean deleted = deleteLocal(key);
            return ResponseEntity.ok(deleted ? "1" : "0");
        } else {
            return forwardDelete(key);
        }
    }

    // Local operations (original methods)
    public String getLocal(String key) {
        Optional<String> value = repository.get(key);
        return value.orElse(null);
    }

    public void setLocal(String key, String value) {
        repository.set(key, value);
    }

    public boolean deleteLocal(String key) {
        return repository.delete(key);
    }

    // Backward compatibility methods
    public String get(String key) {
        return getLocal(key);
    }

    public void set(String key, String value) {
        setLocal(key, value);
    }

    public boolean delete(String key) {
        return deleteLocal(key);
    }

    // Peer forwarding methods
    private ResponseEntity<String> forwardGet(String key) {
        int targetNodeId = getTargetNodeId(key);
        // Convert 1-based nodeId to 0-based array index
        String targetUrl = peerUrls.get(targetNodeId - 1);
        
        try {
            return restTemplate.getForEntity(
                targetUrl + "/api/v1/get/" + key, 
                String.class
            );
        } catch (ResourceAccessException e) {
            throw new RuntimeException("Failed to reach peer node " + targetUrl, e);
        }
    }

    private ResponseEntity<String> forwardSet(String key, String value) {
        int targetNodeId = getTargetNodeId(key);
        // Convert 1-based nodeId to 0-based array index
        String targetUrl = peerUrls.get(targetNodeId - 1);
        
        try {
            SetRequest request = new SetRequest(key, value);
            return restTemplate.postForEntity(
                targetUrl + "/api/v1/set",
                request,
                String.class
            );
        } catch (ResourceAccessException e) {
            throw new RuntimeException("Failed to reach peer node " + targetUrl, e);
        }
    }

    private ResponseEntity<String> forwardDelete(String key) {
        int targetNodeId = getTargetNodeId(key);
        // Convert 1-based nodeId to 0-based array index
        String targetUrl = peerUrls.get(targetNodeId - 1);
        
        try {
            return restTemplate.exchange(
                targetUrl + "/api/v1/del/" + key,
                org.springframework.http.HttpMethod.DELETE,
                null,
                String.class
            );
        } catch (ResourceAccessException e) {
            throw new RuntimeException("Failed to reach peer node " + targetUrl, e);
        }
    }
}