package org.limedb.shard.core.repository.jpa;

import org.limedb.shard.core.model.Entry;
import org.springframework.data.jpa.repository.JpaRepository;
import java.util.Optional;

public interface ShardJpaRepository extends JpaRepository<Entry, Long> {
    Optional<Entry> findByKey(String key);
}