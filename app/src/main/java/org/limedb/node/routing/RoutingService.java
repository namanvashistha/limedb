package org.limedb.node.routing;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;
import jakarta.annotation.PostConstruct;

import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Service that manages consistent hashing routing for the distributed key-value
 * store.
 * Integrates with Spring Boot and provides high-level routing operations.
 */
@Service
public class RoutingService {

    private static final Logger logger = LoggerFactory.getLogger(RoutingService.class);

    private final ConsistentHashRing hashRing;
    private final List<String> peerUrls;
    private final String currentNodeUrl;

    public RoutingService(
            @Value("${node.routing.virtual-nodes:150}") int virtualNodes,
            @Value("#{@peerUrls}") List<String> peerUrls,
            @Value("${server.port:7001}") int serverPort) {

        this.hashRing = new ConsistentHashRing(virtualNodes);
        this.peerUrls = peerUrls;
        this.currentNodeUrl = "http://localhost:" + serverPort;

        logger.info("RoutingService initialized with {} virtual nodes per physical node", virtualNodes);
    }

    /**
     * Initialize the hash ring with all peer nodes
     */
    @PostConstruct
    public void initializeRing() {
        hashRing.initializeRing(peerUrls);
        logger.info("Hash ring initialized with {} nodes: {}", peerUrls.size(), peerUrls);

        // Log ring statistics
        Map<String, Object> stats = hashRing.getRingStats();
        logger.info("Ring stats: {}", stats);
    }

    /**
     * Get the target node URL for a given key
     */
    public String getTargetNodeUrl(String key) {
        String targetUrl = hashRing.getNode(key);
        logger.debug("Key '{}' routes to node: {}", key, targetUrl);
        return targetUrl;
    }

    /**
     * Check if the current node should handle this key locally
     */
    public boolean shouldHandleLocally(String key) {
        String targetUrl = getTargetNodeUrl(key);
        boolean isLocal = currentNodeUrl.equals(targetUrl);
        logger.debug("Key '{}' should handle locally: {} (target: {}, current: {})",
                key, isLocal, targetUrl, currentNodeUrl);
        return isLocal;
    }

    /**
     * Get all nodes in the ring
     */
    public Set<String> getAllNodes() {
        return hashRing.getNodes();
    }

    /**
     * Get the current node's URL
     */
    public String getCurrentNodeUrl() {
        return currentNodeUrl;
    }

    /**
     * Add a new node to the ring (for dynamic scaling)
     */
    public void addNode(String nodeUrl) {
        hashRing.addNode(nodeUrl);
        logger.info("Added node to ring: {}", nodeUrl);

        Map<String, Object> stats = hashRing.getRingStats();
        logger.info("Updated ring stats: {}", stats);
    }

    /**
     * Remove a node from the ring (for dynamic scaling or failure handling)
     */
    public void removeNode(String nodeUrl) {
        hashRing.removeNode(nodeUrl);
        logger.info("Removed node from ring: {}", nodeUrl);

        Map<String, Object> stats = hashRing.getRingStats();
        logger.info("Updated ring stats: {}", stats);
    }

    /**
     * Get detailed ring statistics for monitoring
     */
    public Map<String, Object> getRingStatistics() {
        return hashRing.getRingStats();
    }

    /**
     * Update the ring topology with a new set of nodes
     * (useful for handling cluster membership changes)
     */
    public void updateTopology(List<String> newPeerUrls) {
        logger.info("Updating ring topology. Old nodes: {}, New nodes: {}",
                hashRing.getNodes(), newPeerUrls);

        hashRing.initializeRing(newPeerUrls);

        logger.info("Ring topology updated successfully");
        Map<String, Object> stats = hashRing.getRingStats();
        logger.info("New ring stats: {}", stats);
    }
}