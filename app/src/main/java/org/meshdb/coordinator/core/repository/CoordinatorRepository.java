package org.meshdb.coordinator.core.repository;

import org.meshdb.coordinator.core.model.Coordinator;
import java.util.List;
import java.util.Optional;

public interface CoordinatorRepository {
    List<Coordinator> findAll();
    Optional<Coordinator> findById(Long id);
    Coordinator save(Coordinator coordinator);
    void deleteById(Long id);
    void markAsCompleted(Long id);
}