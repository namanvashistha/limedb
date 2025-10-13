package org.limedb.node.repository.jpa;

import org.limedb.node.model.Entry;
import org.springframework.data.jpa.repository.JpaRepository;
import java.util.Optional;

public interface NodeJpaRepository extends JpaRepository<Entry, Long> {
    Optional<Entry> findByKey(String key);
}