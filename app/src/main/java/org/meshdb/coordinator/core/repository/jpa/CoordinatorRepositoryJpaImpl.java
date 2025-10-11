package org.meshdb.coordinator.core.repository.jpa;

import org.meshdb.coordinator.core.model.Coordinator;
import org.meshdb.coordinator.core.repository.CoordinatorRepository;
import org.springframework.stereotype.Repository;

import java.util.List;
import java.util.Optional;

@Repository  // Marks this as the actual bean Spring will inject
public class CoordinatorRepositoryJpaImpl implements CoordinatorRepository {

    private final CoordinatorJpaRepository jpaRepository;

    public CoordinatorRepositoryJpaImpl(CoordinatorJpaRepository jpaRepository) {
        this.jpaRepository = jpaRepository;
    }

    @Override
    public List<Coordinator> findAll() {
        return jpaRepository.findAll();
    }

    @Override
    public Optional<Coordinator> findById(Long id) {
        return jpaRepository.findById(id);
    }

    @Override
    public Coordinator save(Coordinator coordinator) {
        return jpaRepository.save(coordinator);
    }

    @Override
    public void deleteById(Long id) {
        jpaRepository.deleteById(id);
    }

    @Override
    public void markAsCompleted(Long id) {
        Optional<Coordinator> coordinatorOptional = jpaRepository.findById(id);
        if (coordinatorOptional.isPresent()) {
            Coordinator coordinator = coordinatorOptional.get();
            coordinator.setCompleted(true);
            jpaRepository.save(coordinator);
        }
    }
}