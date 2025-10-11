package org.meshdb.coordinator.core.mapper;


import org.springframework.stereotype.Component;
import org.meshdb.coordinator.core.dto.CoordinatorRequest;
import org.meshdb.coordinator.core.dto.CoordinatorResponse;
import org.meshdb.coordinator.core.model.Coordinator;

@Component
public class CoordinatorMapper {

    public Coordinator toEntity(CoordinatorRequest request) {
        Coordinator coordinator = new Coordinator();
        coordinator.setTitle(request.title());
        coordinator.setCompleted(request.completed());
        return coordinator;
    }

    public CoordinatorResponse toResponse(Coordinator coordinator) {
        return new CoordinatorResponse(coordinator.getId(), coordinator.getTitle(), coordinator.isCompleted());
    }
}