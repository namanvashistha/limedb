package org.meshdb.shard.core.mapper;


import org.springframework.stereotype.Component;
import org.meshdb.shard.core.dto.ShardRequest;
import org.meshdb.shard.core.dto.ShardResponse;
import org.meshdb.shard.core.model.Shard;

@Component
public class ShardMapper {

    public Shard toEntity(ShardRequest request) {
        Shard shard = new Shard();
        shard.setTitle(request.title());
        shard.setCompleted(request.completed());
        return shard;
    }

    public ShardResponse toShardResponse(Shard shard) {
        return new ShardResponse(shard.getId(), shard.getTitle(), shard.isCompleted());
    }
}