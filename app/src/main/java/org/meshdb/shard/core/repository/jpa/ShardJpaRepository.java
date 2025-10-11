package org.meshdb.shard.core.repository.jpa;

import org.meshdb.shard.core.model.Shard;
import org.springframework.data.jpa.repository.JpaRepository;

public interface ShardJpaRepository extends JpaRepository<Shard, Long> {
    // If needed, you can add custom JPA queries here later
}