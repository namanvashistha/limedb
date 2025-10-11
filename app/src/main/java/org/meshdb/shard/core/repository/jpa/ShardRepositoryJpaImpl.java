package org.meshdb.shard.core.repository.jpa;

import org.meshdb.shard.core.model.Shard;
import org.meshdb.shard.core.repository.ShardRepository;
import org.springframework.stereotype.Repository;

import java.util.List;
import java.util.Optional;

@Repository  // Marks this as the actual bean Spring will inject
public class ShardRepositoryJpaImpl implements ShardRepository {

    private final ShardJpaRepository jpaRepository;

    public ShardRepositoryJpaImpl(ShardJpaRepository jpaRepository) {
        this.jpaRepository = jpaRepository;
    }

    @Override
    public List<Shard> findAll() {
        return jpaRepository.findAll();
    }

    @Override
    public Optional<Shard> findById(Long id) {
        return jpaRepository.findById(id);
    }

    @Override
    public Shard save(Shard shard) {
        return jpaRepository.save(shard);
    }

    @Override
    public void deleteById(Long id) {
        jpaRepository.deleteById(id);
    }

    @Override
    public void markAsCompleted(Long id) {
        Optional<Shard> shardOptional = jpaRepository.findById(id);
        if (shardOptional.isPresent()) {
            Shard shard = shardOptional.get();
            shard.setCompleted(true);
            jpaRepository.save(shard);
        }
    }
}