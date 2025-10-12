package org.limedb;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.context.ConfigurableApplicationContext;
import org.springframework.core.env.Environment;

@SpringBootApplication
public class App {
    public static void main(String[] args) {
        ConfigurableApplicationContext context = SpringApplication.run(App.class, args);

        Environment env = context.getEnvironment();
        String nodeId = env.getProperty("node.id", "1");
        String serverPort = env.getProperty("server.port", "7001");
        String peerUrls = env.getProperty("node.peers", "");

        System.out.println("🚀 Starting LimeDB Peer-to-Peer Node");
        System.out.println("Node ID: " + nodeId);
        System.out.println("Port: " + serverPort);
        System.out.println("Peers: " + peerUrls);
        System.out.println("Running in Peer-to-Peer mode - can handle requests and route to other nodes");
    }
}