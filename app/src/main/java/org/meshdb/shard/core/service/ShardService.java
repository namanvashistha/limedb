package org.meshdb.shard.core.service;

import org.meshdb.shard.core.repository.ShardRepository;
import org.springframework.stereotype.Service;

import java.util.Optional;

@Service
public class ShardService {

    private final ShardRepository repository;

    public ShardService(ShardRepository repository) {
        this.repository = repository;
    }

    public String get(String key) {
        Optional<String> value = repository.get(key);
        return value.orElse(null);
    }

    public void set(String key, String value) {
        repository.set(key, value);
    }

    public boolean delete(String key) {
        return repository.delete(key);
    }
}