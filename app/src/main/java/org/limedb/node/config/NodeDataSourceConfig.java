package org.limedb.node.config;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.jdbc.DataSourceBuilder;
import org.springframework.boot.web.client.RestTemplateBuilder;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.context.annotation.Primary;
import org.springframework.web.client.RestTemplate;

import javax.sql.DataSource;
import java.time.Duration;
import java.util.Arrays;
import java.util.List;

@Configuration
public class NodeDataSourceConfig {

    @Value("${node.id:1}")
    private int nodeId;

    @Value("${node.peers:http://localhost:7001,http://localhost:7002,http://localhost:7003}")
    private String peers;

    @Value("${spring.datasource.username:limedb}")
    private String username;

    @Value("${spring.datasource.password:limedb}")
    private String password;

    @Value("${spring.datasource.host:localhost}")
    private String host;

    @Value("${spring.datasource.port:5432}")
    private String port;

    @Bean
    @Primary
    public DataSource dataSource() {
        // Database name: direct mapping since nodeId is 1-based
        String databaseName = "limedb_node_" + nodeId;
        String url = String.format("jdbc:postgresql://%s:%s/%s", host, port, databaseName);
        
        return DataSourceBuilder.create()
                .driverClassName("org.postgresql.Driver")
                .url(url)
                .username(username)
                .password(password)
                .build();
    }

    @Bean
    public RestTemplate restTemplate(RestTemplateBuilder builder) {
        return builder
                .connectTimeout(Duration.ofSeconds(5))
                .readTimeout(Duration.ofSeconds(10))
                .build();
    }

    @Bean
    public int nodeId() {
        return nodeId;
    }

    @Bean
    public List<String> peerUrls() {
        return Arrays.asList(peers.split(","));
    }
}
