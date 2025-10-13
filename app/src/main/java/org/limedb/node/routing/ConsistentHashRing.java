package org.limedb.node.routing;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.*;
import java.util.concurrent.ConcurrentSkipListMap;

/**
 * Thread-safe consistent hash ring implementation for distributed key-value
 * storage.
 * Uses virtual nodes to ensure better load distribution across physical nodes.
 */
public class ConsistentHashRing {

    private final ConcurrentSkipListMap<Long, String> ring;
    private final int virtualNodesPerNode;
    private final MessageDigest md5;
    private final Set<String> nodes;

    public ConsistentHashRing(int virtualNodesPerNode) {
        this.ring = new ConcurrentSkipListMap<>();
        this.virtualNodesPerNode = virtualNodesPerNode;
        this.nodes = new HashSet<>();

        try {
            this.md5 = MessageDigest.getInstance("MD5");
        } catch (NoSuchAlgorithmException e) {
            throw new RuntimeException("MD5 algorithm not available", e);
        }
    }

    /**
     * Add a node to the hash ring with virtual nodes for better distribution
     */
    public synchronized void addNode(String nodeUrl) {
        if (nodes.contains(nodeUrl)) {
            return; // Node already exists
        }

        nodes.add(nodeUrl);

        // Add virtual nodes for better distribution
        for (int i = 0; i < virtualNodesPerNode; i++) {
            String virtualNodeKey = nodeUrl + ":" + i;
            long hash = hash(virtualNodeKey);
            ring.put(hash, nodeUrl);
        }
    }

    /**
     * Remove a node from the hash ring
     */
    public synchronized void removeNode(String nodeUrl) {
        if (!nodes.contains(nodeUrl)) {
            return; // Node doesn't exist
        }

        nodes.remove(nodeUrl);

        // Remove all virtual nodes for this physical node
        for (int i = 0; i < virtualNodesPerNode; i++) {
            String virtualNodeKey = nodeUrl + ":" + i;
            long hash = hash(virtualNodeKey);
            ring.remove(hash);
        }
    }

    /**
     * Get the node responsible for a given key.
     * Returns the first node clockwise from the key's hash position.
     */
    public String getNode(String key) {
        if (ring.isEmpty()) {
            return null;
        }

        long keyHash = hash(key);

        // Find the first node with hash >= keyHash (clockwise)
        Map.Entry<Long, String> entry = ring.ceilingEntry(keyHash);

        if (entry == null) {
            // Wrap around to the first node in the ring
            entry = ring.firstEntry();
        }

        return entry.getValue();
    }

    /**
     * Get all nodes currently in the ring
     */
    public synchronized Set<String> getNodes() {
        return new HashSet<>(nodes);
    }

    /**
     * Get the number of nodes in the ring
     */
    public int getNodeCount() {
        return nodes.size();
    }

    /**
     * Initialize the ring with a list of node URLs
     */
    public synchronized void initializeRing(List<String> nodeUrls) {
        ring.clear();
        nodes.clear();

        for (String nodeUrl : nodeUrls) {
            addNode(nodeUrl);
        }
    }

    /**
     * Get ring statistics for monitoring and debugging
     */
    public synchronized Map<String, Object> getRingStats() {
        Map<String, Object> stats = new HashMap<>();
        stats.put("totalNodes", nodes.size());
        stats.put("virtualNodes", ring.size());
        stats.put("virtualNodesPerNode", virtualNodesPerNode);

        // Calculate distribution
        Map<String, Integer> distribution = new HashMap<>();
        for (String node : ring.values()) {
            distribution.put(node, distribution.getOrDefault(node, 0) + 1);
        }
        stats.put("virtualNodeDistribution", distribution);

        return stats;
    }

    /**
     * Hash function using MD5
     */
    private long hash(String input) {
        md5.reset();
        md5.update(input.getBytes(StandardCharsets.UTF_8));
        byte[] digest = md5.digest();

        // Convert first 8 bytes to long for hash value
        long hash = 0;
        for (int i = 0; i < 8; i++) {
            hash = (hash << 8) | (digest[i] & 0xFF);
        }

        return hash;
    }
}