package org.meshdb.shard.core.repository;

import org.meshdb.shard.core.model.Shard;
import java.util.List;
import java.util.Optional;

public interface ShardRepository {
    List<Shard> findAll();
    Optional<Shard> findById(Long id);
    Shard save(Shard shard);
    void deleteById(Long id);
    void markAsCompleted(Long id);
}