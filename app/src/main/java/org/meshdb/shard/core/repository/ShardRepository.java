package org.meshdb.shard.core.repository;

import java.util.List;
import java.util.Optional;

public interface ShardRepository {
    Optional<String> get(String key);
    void set(String key, String value);
    boolean delete(String key);
}