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
        String nodeType = env.getProperty("node.type", "coordinator");

        System.out.println("🚀 Starting LimeDB Node Type: " + nodeType.toUpperCase());
        System.out.println("On Port: " + env.getProperty("server.port", "8080"));

        if (nodeType.equalsIgnoreCase("shard")) {
            System.out.println("Running Shard Server...");
        } else if (nodeType.equalsIgnoreCase("coordinator")) {
            System.out.println("Running Coordinator Server...");
        } else {
            throw new IllegalArgumentException("Invalid node.type: " + nodeType);
        }
    }
}