package org.limedb.node.core.repository.jpa;

import org.limedb.node.core.model.Entry;
import org.springframework.data.jpa.repository.JpaRepository;
import java.util.Optional;

public interface NodeJpaRepository extends JpaRepository<Entry, Long> {
    Optional<Entry> findByKey(String key);
}