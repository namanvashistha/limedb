package org.meshdb.shard.core.service;

import org.meshdb.shard.core.dto.ShardRequest;
import org.meshdb.shard.core.dto.ShardResponse;
import org.meshdb.shard.core.mapper.ShardMapper;
import org.meshdb.shard.core.model.Shard;
import org.meshdb.shard.core.repository.ShardRepository;
import org.springframework.stereotype.Service;

import java.util.List;

@Service
public class ShardService {

    private final ShardRepository repository;
    private final ShardMapper mapper;

    public ShardService(ShardRepository repository, ShardMapper mapper) {
        this.repository = repository;
        this.mapper = mapper;
    }

    public List<ShardResponse> getAll() {
        return repository.findAll().stream()
                .map(mapper::toShardResponse)
                .toList();
    }

    public ShardResponse create(ShardRequest request) {
        Shard shard = mapper.toEntity(request);
        return mapper.toShardResponse(repository.save(shard));
    }

    public ShardResponse update(Long id, ShardRequest request) {
        Shard existing = repository.findById(id)
                .orElseThrow(() -> new RuntimeException("Shard not found"));
        existing.setTitle(request.title());
        existing.setCompleted(request.completed());
        return mapper.toShardResponse(repository.save(existing));
    }

    public void delete(Long id) {
        repository.deleteById(id);
    }

    public ShardResponse markAsCompleted(Long id) {
        Shard shard = repository.findById(id)
                .orElseThrow(() -> new RuntimeException("Shard not found"));
        repository.markAsCompleted(id);
        // Fetch the updated shard to return the ShardResponse
        Shard updatedShard = repository.findById(id)
                .orElseThrow(() -> new RuntimeException("Shard not found"));
        return mapper.toShardResponse(updatedShard);
    }
}