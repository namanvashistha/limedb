package org.meshdb.coordinator.core.repository.jpa;

import org.meshdb.coordinator.core.model.Coordinator;
import org.springframework.data.jpa.repository.JpaRepository;

public interface CoordinatorJpaRepository extends JpaRepository<Coordinator, Long> {
    // If needed, you can add custom JPA queries here later
}