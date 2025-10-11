package org.meshdb.coordinator.core.service;

import org.meshdb.coordinator.core.dto.CoordinatorRequest;
import org.meshdb.coordinator.core.dto.CoordinatorResponse;
import org.meshdb.coordinator.core.mapper.CoordinatorMapper;
import org.meshdb.coordinator.core.model.Coordinator;
import org.meshdb.coordinator.core.repository.CoordinatorRepository;
import org.springframework.stereotype.Service;

import java.util.List;

@Service
public class CoordinatorService {

    private final CoordinatorRepository repository;
    private final CoordinatorMapper mapper;

    public CoordinatorService(CoordinatorRepository repository, CoordinatorMapper mapper) {
        this.repository = repository;
        this.mapper = mapper;
    }

    public List<CoordinatorResponse> getAll() {
        return repository.findAll().stream()
                .map(mapper::toResponse)
                .toList();
    }

    public CoordinatorResponse create(CoordinatorRequest request) {
        Coordinator coordinator = mapper.toEntity(request);
        return mapper.toResponse(repository.save(coordinator));
    }

    public CoordinatorResponse update(Long id, CoordinatorRequest request) {
        Coordinator existing = repository.findById(id)
                .orElseThrow(() -> new RuntimeException("Coordinator not found"));
        existing.setTitle(request.title());
        existing.setCompleted(request.completed());
        return mapper.toResponse(repository.save(existing));
    }

    public void delete(Long id) {
        repository.deleteById(id);
    }

    public CoordinatorResponse markAsCompleted(Long id) {
        Coordinator coordinator = repository.findById(id)
                .orElseThrow(() -> new RuntimeException("Coordinator not found"));
        repository.markAsCompleted(id);
        // Fetch the updated coordinator to return the CoordinatorResponse
        Coordinator updatedCoordinator = repository.findById(id)
                .orElseThrow(() -> new RuntimeException("Coordinator not found"));
        return mapper.toResponse(updatedCoordinator);
    }
}