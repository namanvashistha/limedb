package org.limedb.shard.core.repository.jpa;

import org.limedb.shard.core.model.Entry;
import org.limedb.shard.core.repository.ShardRepository;
import org.springframework.stereotype.Repository;

import java.util.Optional;

@Repository
public class ShardRepositoryJpaImpl implements ShardRepository {

    private final ShardJpaRepository jpaRepository;

    public ShardRepositoryJpaImpl(ShardJpaRepository jpaRepository) {
        this.jpaRepository = jpaRepository;
    }

    @Override
    public Optional<String> get(String key) {
        Optional<Entry> entry = jpaRepository.findByKey(key);
        return entry.map(Entry::getValue);
    }

    @Override
    public void set(String key, String value) {
        Optional<Entry> existingEntry = jpaRepository.findByKey(key);
        if (existingEntry.isPresent()) {
            // Update existing
            Entry entry = existingEntry.get();
            entry.setValue(value);
            jpaRepository.save(entry);
        } else {
            // Create new
            Entry entry = new Entry();
            entry.setKey(key);
            entry.setValue(value);
            jpaRepository.save(entry);
        }
    }

    @Override
    public boolean delete(String key) {
        Optional<Entry> entry = jpaRepository.findByKey(key);
        if (entry.isPresent()) {
            jpaRepository.delete(entry.get());
            return true;
        }
        return false;
    }
}