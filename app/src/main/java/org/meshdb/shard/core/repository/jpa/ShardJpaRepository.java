package org.meshdb.shard.core.repository.jpa;

import org.meshdb.shard.core.model.Entry;
import org.springframework.data.jpa.repository.JpaRepository;
import java.util.Optional;

public interface ShardJpaRepository extends JpaRepository<Entry, Long> {
    Optional<Entry> findByKey(String key);
}