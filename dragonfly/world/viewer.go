package world

import (
	blockAction "github.com/df-mc/dragonfly/dragonfly/block/action"
	"github.com/df-mc/dragonfly/dragonfly/entity/action"
	"github.com/df-mc/dragonfly/dragonfly/entity/state"
	"github.com/df-mc/dragonfly/dragonfly/world/chunk"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/google/uuid"
)

// Viewer is a viewer in the world. It can view changes that are made in the world, such as the addition of
// entities and the changes of blocks.
type Viewer interface {
	// Position returns the position of the viewer.
	Position() mgl64.Vec3
	// ViewEntity views the entity passed. It is called for every entity that the viewer may encounter in the
	// world, either by moving entities or by moving the viewer using a world.Loader.
	ViewEntity(e Entity)
	// HideEntity stops viewing the entity passed. It is called for every entity that leaves the viewing range
	// of the viewer, either by its movement or the movement of the viewer using a world.Loader.
	HideEntity(e Entity)
	// ViewEntityMovement views the movement of an entity. The entity is moved with a delta position, yaw and
	// pitch, which, when applied to values of the entity, will result in the final values.
	ViewEntityMovement(e Entity, deltaPos mgl64.Vec3, deltaYaw, deltaPitch float64)
	// ViewEntityVelocity views the velocity of an entity. It is called right before a call to
	// ViewEntityMovement so that the Viewer may interpolate the movement itself.
	ViewEntityVelocity(e Entity, velocity mgl64.Vec3)
	// ViewEntityTeleport views the teleportation of an entity. The entity is immediately moved to a different
	// target position.
	ViewEntityTeleport(e Entity, position mgl64.Vec3)
	// ViewEntitySkin updates the skin of an entity.
	ViewEntitySkin(e Entity)
	// ViewChunk views the chunk passed at a particular position. It is called for every chunk loaded using
	// the world.Loader.
	ViewChunk(pos ChunkPos, c *chunk.Chunk, blockNBT map[BlockPos]Block)
	// ViewTime views the time of the world. It is called every time the time is changed or otherwise every
	// second.
	ViewTime(time int)
	// ViewEntityItems views the items currently held by an entity that is able to equip items.
	ViewEntityItems(e Entity)
	// ViewEntityArmour views the items currently equipped as armour by the entity.
	ViewEntityArmour(e Entity)
	// ViewEntityAction views an action performed by an entity. Available actions may be found in the `action`
	// package, and include things such as swinging an arm.
	ViewEntityAction(e Entity, a action.Action)
	// ViewEntityState views the current state of an entity. It is called whenever an entity changes its
	// physical appearance, for example when sprinting.
	ViewEntityState(e Entity, s []state.State)
	// ViewParticle views a particle spawned at a given position in the world. It is called when a particle,
	// for example a block breaking particle, is spawned near the player.
	ViewParticle(pos mgl64.Vec3, p Particle)
	// ViewSound is called when a sound is played in the world.
	ViewSound(pos mgl64.Vec3, s Sound)
	// ViewBlockUpdate views the updating of a block. It is called when a block is set at the position passed
	// to the method.
	ViewBlockUpdate(pos BlockPos, b Block, layer int)
	// ViewBlockAction views an action performed by a block. Available actions may be found in the `action`
	// package, and include things such as a chest opening.
	ViewBlockAction(pos BlockPos, a blockAction.Action)
	// ViewEmote views an emote being performed by another entity.
	ViewEmote(player Entity, emote uuid.UUID)
}
