package org.limedb.shard.config;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.jdbc.DataSourceBuilder;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.context.annotation.Primary;

import javax.sql.DataSource;

@Configuration
public class ShardDataSourceConfig {

    @Value("${shard.id:1}")
    private String shardId;

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
        String databaseName = "limedb_shard_" + shardId;
        String url = String.format("jdbc:postgresql://%s:%s/%s", host, port, databaseName);
        
        return DataSourceBuilder.create()
                .driverClassName("org.postgresql.Driver")
                .url(url)
                .username(username)
                .password(password)
                .build();
    }
}
