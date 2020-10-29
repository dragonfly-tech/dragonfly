package session

import (
	"bytes"
	"github.com/cespare/xxhash"
	"github.com/df-mc/dragonfly/dragonfly/block"
	blockAction "github.com/df-mc/dragonfly/dragonfly/block/action"
	"github.com/df-mc/dragonfly/dragonfly/entity"
	"github.com/df-mc/dragonfly/dragonfly/entity/action"
	"github.com/df-mc/dragonfly/dragonfly/entity/state"
	"github.com/df-mc/dragonfly/dragonfly/item"
	"github.com/df-mc/dragonfly/dragonfly/item/inventory"
	"github.com/df-mc/dragonfly/dragonfly/world"
	"github.com/df-mc/dragonfly/dragonfly/world/chunk"
	"github.com/df-mc/dragonfly/dragonfly/world/particle"
	"github.com/df-mc/dragonfly/dragonfly/world/sound"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/nbt"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// ViewChunk ...
func (s *Session) ViewChunk(pos world.ChunkPos, c *chunk.Chunk, blockEntities map[world.BlockPos]world.Block) {
	if !s.conn.ClientCacheEnabled() {
		s.sendNetworkChunk(pos, c, blockEntities)
		return
	}
	s.sendBlobHashes(pos, c, blockEntities)
}

// sendBlobHashes sends chunk blob hashes of the data of the chunk and stores the data in a map of blobs. Only
// data that the client doesn't yet have will be sent over the network.
func (s *Session) sendBlobHashes(pos world.ChunkPos, c *chunk.Chunk, blockEntities map[world.BlockPos]world.Block) {
	data := chunk.DiskEncode(c, true)

	count := byte(0)
	for y := byte(0); y < 16; y++ {
		if data.SubChunks[y] != nil {
			count = y + 1
		}
	}

	blobs := make([][]byte, 0, count+1)
	for y := byte(0); y < count; y++ {
		if data.SubChunks[y] == nil {
			blobs = append(blobs, []byte{chunk.SubChunkVersion, 0})
			continue
		}
		blobs = append(blobs, data.SubChunks[y])
	}
	blobs = append(blobs, data.Data2D[512:])

	m := make(map[uint64]struct{}, len(blobs))
	hashes := make([]uint64, len(blobs))
	for i, blob := range blobs {
		h := xxhash.Sum64(blob)
		hashes[i] = h
		m[h] = struct{}{}
	}

	s.blobMu.Lock()
	s.openChunkTransactions = append(s.openChunkTransactions, m)
	if len(s.blobs) > 4096 {
		s.blobMu.Unlock()
		s.log.Errorf("player %v has too many blobs pending %v: disconnecting", s.c.Name(), len(s.blobs))
		_ = s.c.Close()
		return
	}
	for i, hash := range hashes {
		s.blobs[hash] = blobs[i]
	}
	s.blobMu.Unlock()

	raw := bytes.NewBuffer(make([]byte, 1, 32))
	enc := nbt.NewEncoderWithEncoding(raw, nbt.NetworkLittleEndian)
	for pos, b := range blockEntities {
		if n, ok := b.(world.NBTer); ok {
			data := n.EncodeNBT()
			data["x"], data["y"], data["z"] = int32(pos[0]), int32(pos[1]), int32(pos[2])
			_ = enc.Encode(data)
		}
	}

	s.writePacket(&packet.LevelChunk{
		ChunkX:        pos[0],
		ChunkZ:        pos[1],
		SubChunkCount: uint32(count),
		CacheEnabled:  true,
		BlobHashes:    hashes,
		RawPayload:    raw.Bytes(),
	})
}

// sendNetworkChunk sends a network encoded chunk to the client.
func (s *Session) sendNetworkChunk(pos world.ChunkPos, c *chunk.Chunk, blockEntities map[world.BlockPos]world.Block) {
	data := chunk.NetworkEncode(c)

	count := byte(0)
	for y := byte(0); y < 16; y++ {
		if data.SubChunks[y] != nil {
			count = y + 1
		}
	}
	for y := byte(0); y < count; y++ {
		if data.SubChunks[y] == nil {
			_ = s.chunkBuf.WriteByte(chunk.SubChunkVersion)
			// We write zero here, meaning the sub chunk has no block storages: The sub chunk is completely
			// empty.
			_ = s.chunkBuf.WriteByte(0)
			continue
		}
		_, _ = s.chunkBuf.Write(data.SubChunks[y])
	}
	_, _ = s.chunkBuf.Write(data.Data2D)
	_, _ = s.chunkBuf.Write(data.BlockNBT)

	enc := nbt.NewEncoderWithEncoding(s.chunkBuf, nbt.NetworkLittleEndian)
	for pos, b := range blockEntities {
		if n, ok := b.(world.NBTer); ok {
			data := n.EncodeNBT()
			data["x"], data["y"], data["z"] = int32(pos[0]), int32(pos[1]), int32(pos[2])
			_ = enc.Encode(data)
		}
	}

	s.writePacket(&packet.LevelChunk{
		ChunkX:        pos[0],
		ChunkZ:        pos[1],
		SubChunkCount: uint32(count),
		RawPayload:    append([]byte(nil), s.chunkBuf.Bytes()...),
	})
	s.chunkBuf.Reset()
}

// ViewEntity ...
func (s *Session) ViewEntity(e world.Entity) {
	if s.entityRuntimeID(e) == selfEntityRuntimeID {
		return
	}
	var runtimeID uint64

	s.entityMutex.Lock()
	_, controllable := e.(Controllable)

	if id, ok := s.entityRuntimeIDs[e]; ok && controllable {
		runtimeID = id
	} else {
		runtimeID = s.currentEntityRuntimeID.Add(1)
		s.entityRuntimeIDs[e] = runtimeID
		s.entities[runtimeID] = e
	}
	s.entityMutex.Unlock()

	switch v := e.(type) {
	case Controllable:
		s.writePacket(&packet.AddPlayer{
			UUID:            v.UUID(),
			Username:        v.Name(),
			EntityUniqueID:  int64(runtimeID),
			EntityRuntimeID: runtimeID,
			Position:        vec64To32(e.Position()),
			Pitch:           float32(e.Pitch()),
			Yaw:             float32(e.Yaw()),
			HeadYaw:         float32(e.Yaw()),
		})
	case *entity.Item:
		s.writePacket(&packet.AddItemActor{
			EntityUniqueID:  int64(runtimeID),
			EntityRuntimeID: runtimeID,
			Item:            stackFromItem(v.Item()),
			Position:        vec64To32(v.Position()),
		})
	case *entity.FallingBlock:
		s.writePacket(&packet.AddActor{
			EntityUniqueID:  int64(runtimeID),
			EntityRuntimeID: runtimeID,
			EntityType:      "minecraft:falling_block",
			EntityMetadata:  map[uint32]interface{}{dataKeyVariant: int32(s.blockRuntimeID(v.Block()))},
			Position:        vec64To32(e.Position()),
			Pitch:           float32(e.Pitch()),
			Yaw:             float32(e.Yaw()),
			HeadYaw:         float32(e.Yaw()),
		})
	default:
		s.writePacket(&packet.AddActor{
			EntityUniqueID:  int64(runtimeID),
			EntityRuntimeID: runtimeID,
			EntityType:      e.EncodeEntity(),
			Position:        vec64To32(e.Position()),
			Pitch:           float32(e.Pitch()),
			Yaw:             float32(e.Yaw()),
			HeadYaw:         float32(e.Yaw()),
		})
	}
}

// HideEntity ...
func (s *Session) HideEntity(e world.Entity) {
	if s.entityRuntimeID(e) == selfEntityRuntimeID {
		return
	}

	s.entityMutex.Lock()
	id, ok := s.entityRuntimeIDs[e]
	if _, controllable := e.(Controllable); !controllable {
		delete(s.entityRuntimeIDs, e)
		delete(s.entities, s.entityRuntimeIDs[e])
	}
	s.entityMutex.Unlock()
	if !ok {
		// The entity was already removed some other way. We don't need to send a packet.
		return
	}
	s.writePacket(&packet.RemoveActor{EntityUniqueID: int64(id)})
}

// ViewEntityMovement ...
func (s *Session) ViewEntityMovement(e world.Entity, deltaPos mgl64.Vec3, deltaYaw, deltaPitch float64) {
	id := s.entityRuntimeID(e)

	if id == selfEntityRuntimeID {
		return
	}

	switch e.(type) {
	case Controllable:
		s.writePacket(&packet.MovePlayer{
			EntityRuntimeID: id,
			Position:        vec64To32(e.Position().Add(deltaPos).Add(entityOffset(e))),
			Pitch:           float32(e.Pitch() + deltaPitch),
			Yaw:             float32(e.Yaw() + deltaYaw),
			HeadYaw:         float32(e.Yaw() + deltaYaw),
			OnGround:        e.OnGround(),
		})
	default:
		flags := byte(0)
		if e.OnGround() {
			flags |= packet.MoveFlagOnGround
		}
		s.writePacket(&packet.MoveActorAbsolute{
			EntityRuntimeID: id,
			Position:        vec64To32(e.Position().Add(deltaPos).Add(entityOffset(e))),
			Rotation:        vec64To32(mgl64.Vec3{e.Pitch() + deltaPitch, e.Yaw() + deltaYaw}),
			Flags:           flags,
		})
	}
}

// ViewEntityVelocity ...
func (s *Session) ViewEntityVelocity(e world.Entity, velocity mgl64.Vec3) {
	s.writePacket(&packet.SetActorMotion{
		EntityRuntimeID: s.entityRuntimeID(e),
		Velocity:        vec64To32(velocity),
	})
}

// ViewEntitySkin ...
func (s *Session) ViewEntitySkin(e world.Entity) {
	switch player := e.(type) {
	case Controllable:
		s.writePacket(&packet.PlayerSkin{
			UUID: player.UUID(),
			Skin: skinToProtocol(player.Skin()),
		})
	}
}

// entityOffset returns the offset that entities have client-side.
func entityOffset(e world.Entity) mgl64.Vec3 {
	switch e.(type) {
	case Controllable:
		return mgl64.Vec3{0, 1.62}
	case *entity.Item:
		return mgl64.Vec3{0, 0.125}
	case *entity.FallingBlock:
		return mgl64.Vec3{0, 0.49, 0}
	}
	return mgl64.Vec3{}
}

// ViewTime ...
func (s *Session) ViewTime(time int) {
	s.writePacket(&packet.SetTime{Time: int32(time)})
}

// ViewEntityTeleport ...
func (s *Session) ViewEntityTeleport(e world.Entity, position mgl64.Vec3) {
	id := s.entityRuntimeID(e)

	if id == selfEntityRuntimeID {
		s.chunkLoader.Move(position)

		s.teleportMu.Lock()
		s.teleportPos = &position
		s.teleportMu.Unlock()
	}

	switch e.(type) {
	case Controllable:
		s.writePacket(&packet.MovePlayer{
			EntityRuntimeID: id,
			Position:        vec64To32(position.Add(entityOffset(e))),
			Pitch:           float32(e.Pitch()),
			Yaw:             float32(e.Yaw()),
			HeadYaw:         float32(e.Yaw()),
			Mode:            packet.MoveModeTeleport,
		})
	default:
		s.writePacket(&packet.MoveActorAbsolute{
			EntityRuntimeID: id,
			Position:        vec64To32(position.Add(entityOffset(e))),
			Rotation:        vec64To32(mgl64.Vec3{e.Pitch(), e.Yaw()}),
			Flags:           packet.MoveFlagTeleport,
		})
	}
}

// ViewEntityItems ...
func (s *Session) ViewEntityItems(e world.Entity) {
	runtimeID := s.entityRuntimeID(e)
	if runtimeID == selfEntityRuntimeID {
		// Don't view the items of the entity if the entity is the Controllable of the session.
		return
	}
	c, ok := e.(item.Carrier)
	if !ok {
		return
	}

	mainHand, offHand := c.HeldItems()

	// Show the main hand item.
	s.writePacket(&packet.MobEquipment{
		EntityRuntimeID: runtimeID,
		NewItem:         stackFromItem(mainHand),
	})
	// Show the off-hand item.
	s.writePacket(&packet.MobEquipment{
		EntityRuntimeID: runtimeID,
		NewItem:         stackFromItem(offHand),
		WindowID:        protocol.WindowIDOffHand,
	})
}

// ViewEntityArmour ...
func (s *Session) ViewEntityArmour(e world.Entity) {
	runtimeID := s.entityRuntimeID(e)
	if runtimeID == selfEntityRuntimeID {
		// Don't view the items of the entity if the entity is the Controllable of the session.
		return
	}
	armoured, ok := e.(item.Armoured)
	if !ok {
		return
	}

	inv := armoured.Armour()

	// Show the main hand item.
	s.writePacket(&packet.MobArmourEquipment{
		EntityRuntimeID: runtimeID,
		Helmet:          stackFromItem(inv.Helmet()),
		Chestplate:      stackFromItem(inv.Chestplate()),
		Leggings:        stackFromItem(inv.Leggings()),
		Boots:           stackFromItem(inv.Boots()),
	})
}

// ViewParticle ...
func (s *Session) ViewParticle(pos mgl64.Vec3, p world.Particle) {
	switch pa := p.(type) {
	case particle.DragonEggTeleport:
		xSign, ySign, zSign := 0, 0, 0
		if pa.Diff.X() < 0 {
			xSign = 1 << 24
		}
		if pa.Diff.Y() < 0 {
			ySign = 1 << 25
		}
		if pa.Diff.Z() < 0 {
			zSign = 1 << 26
		}

		s.writePacket(&packet.LevelEvent{
			EventType: packet.EventParticleDragonEggTeleport,
			Position:  vec64To32(pos),
			EventData: int32((((((abs(pa.Diff.X()) << 16) | (abs(pa.Diff.Y()) << 8)) | abs(pa.Diff.Z())) | xSign) | ySign) | zSign),
		})
	case particle.Note:
		s.writePacket(&packet.BlockEvent{
			EventType: pa.Instrument.Int32(),
			EventData: int32(pa.Pitch),
			Position:  protocol.BlockPos{int32(pos.X()), int32(pos.Y()), int32(pos.Z())},
		})
	case particle.HugeExplosion:
		s.writePacket(&packet.LevelEvent{
			EventType: packet.EventParticleExplosion,
			Position:  vec64To32(pos),
		})
	case particle.Bonemeal:
		s.writePacket(&packet.LevelEvent{
			EventType: packet.EventParticleCropGrowth,
			Position:  vec64To32(pos),
		})
	case particle.BlockForceField:
		s.writePacket(&packet.LevelEvent{
			EventType: packet.EventParticleBlockForceField,
			Position:  vec64To32(pos),
		})
	case particle.BlockBreak:
		s.writePacket(&packet.LevelEvent{
			EventType: packet.EventParticleDestroy,
			Position:  vec64To32(pos),
			EventData: int32(s.blockRuntimeID(pa.Block)),
		})
	case particle.PunchBlock:
		s.writePacket(&packet.LevelEvent{
			EventType: packet.EventParticlePunchBlock,
			Position:  vec64To32(pos),
			EventData: int32(s.blockRuntimeID(pa.Block)) | (int32(pa.Face) << 24),
		})
	}
}

// ViewSound ...
func (s *Session) ViewSound(pos mgl64.Vec3, soundType world.Sound) {
	pk := &packet.LevelSoundEvent{
		Position:   vec64To32(pos),
		EntityType: ":",
		ExtraData:  -1,
	}
	switch so := soundType.(type) {
	case sound.Note:
		pk.SoundType = packet.SoundEventNote
		pk.ExtraData = (so.Instrument.Int32() << 8) | int32(so.Pitch)
	case sound.DoorCrash:
		s.writePacket(&packet.LevelEvent{
			EventType: packet.EventSoundDoorCrash,
			Position:  vec64To32(pos),
		})
	case sound.Explosion:
		pk.SoundType = packet.SoundEventExplode
	case sound.Click:
		s.writePacket(&packet.LevelEvent{
			EventType: packet.EventSoundClick,
			Position:  vec64To32(pos),
		})
	case sound.Pop:
		s.writePacket(&packet.LevelEvent{
			EventType: packet.EventSoundPop,
			Position:  vec64To32(pos),
		})
	case sound.FireExtinguish:
		pk.SoundType = packet.SoundEventExtinguishFire
	case sound.Ignite:
		pk.SoundType = packet.SoundEventIgnite
	case sound.Burp:
		pk.SoundType = packet.SoundEventBurp
	case sound.Door:
		s.writePacket(&packet.LevelEvent{
			EventType: packet.EventSoundDoor,
			Position:  vec64To32(pos),
		})
		return
	case sound.Deny:
		pk.SoundType = packet.SoundEventDeny
	case sound.BlockPlace:
		pk.SoundType, pk.ExtraData = packet.SoundEventPlace, int32(s.blockRuntimeID(so.Block))
	case sound.ChestClose:
		pk.SoundType = packet.SoundEventChestClosed
	case sound.ChestOpen:
		pk.SoundType = packet.SoundEventChestOpen
	case sound.BlockBreaking:
		pk.SoundType, pk.ExtraData = packet.SoundEventHit, int32(s.blockRuntimeID(so.Block))
	case sound.ItemBreak:
		pk.SoundType = packet.SoundEventBreak
	case sound.ItemUseOn:
		pk.SoundType, pk.ExtraData = packet.SoundEventItemUseOn, int32(s.blockRuntimeID(so.Block))
	case sound.Fizz:
		pk.SoundType = packet.SoundEventFizz
	case sound.Attack:
		pk.SoundType, pk.EntityType = packet.SoundEventAttackStrong, "minecraft:player"
		if !so.Damage {
			pk.SoundType = packet.SoundEventAttackNoDamage
		}
	case sound.BucketFill:
		if _, water := so.Liquid.(block.Water); water {
			pk.SoundType = packet.SoundEventBucketFillWater
			break
		}
		pk.SoundType = packet.SoundEventBucketFillLava
	case sound.BucketEmpty:
		if _, water := so.Liquid.(block.Water); water {
			pk.SoundType = packet.SoundEventBucketEmptyWater
			break
		}
		pk.SoundType = packet.SoundEventBucketEmptyLava
	}
	s.writePacket(pk)
}

// ViewBlockUpdate ...
func (s *Session) ViewBlockUpdate(pos world.BlockPos, b world.Block, layer int) {
	runtimeID, _ := world.BlockRuntimeID(b)
	blockPos := protocol.BlockPos{int32(pos[0]), int32(pos[1]), int32(pos[2])}
	s.writePacket(&packet.UpdateBlock{
		Position:          blockPos,
		NewBlockRuntimeID: runtimeID,
		Flags:             packet.BlockUpdateNetwork,
		Layer:             uint32(layer),
	})
	if v, ok := b.(world.NBTer); ok {
		s.writePacket(&packet.BlockActorData{
			Position: blockPos,
			NBTData:  v.EncodeNBT(),
		})
	}
}

// ViewEntityAction ...
func (s *Session) ViewEntityAction(e world.Entity, a action.Action) {
	switch act := a.(type) {
	case action.SwingArm:
		if _, ok := e.(Controllable); ok {
			if s.entityRuntimeID(e) == selfEntityRuntimeID && s.swingingArm.Load() {
				return
			}
			s.writePacket(&packet.Animate{
				ActionType:      packet.AnimateActionSwingArm,
				EntityRuntimeID: s.entityRuntimeID(e),
			})
			return
		}
		s.writePacket(&packet.ActorEvent{
			EntityRuntimeID: s.entityRuntimeID(e),
			EventType:       packet.ActorEventStartAttack,
		})
	case action.Hurt:
		s.writePacket(&packet.ActorEvent{
			EntityRuntimeID: s.entityRuntimeID(e),
			EventType:       packet.ActorEventHurt,
		})
	case action.Death:
		s.writePacket(&packet.ActorEvent{
			EntityRuntimeID: s.entityRuntimeID(e),
			EventType:       packet.ActorEventDeath,
		})
	case action.PickedUp:
		s.writePacket(&packet.TakeItemActor{
			ItemEntityRuntimeID:  s.entityRuntimeID(e),
			TakerEntityRuntimeID: s.entityRuntimeID(act.Collector.(world.Entity)),
		})
	case action.Eat:
		if user, ok := e.(item.User); ok {
			held, _ := user.HeldItems()
			id, meta := held.Item().(world.Item).EncodeItem()
			s.writePacket(&packet.ActorEvent{
				EntityRuntimeID: s.entityRuntimeID(e),
				EventType:       packet.ActorEventEatingItem,
				EventData:       (id << 16) | int32(meta),
			})
		}
	}
}

// ViewEntityState ...
func (s *Session) ViewEntityState(e world.Entity, states []state.State) {
	m := defaultEntityMetadata(e)
	for _, eState := range states {
		switch st := eState.(type) {
		case state.Sneaking:
			m.setFlag(dataKeyFlags, dataFlagSneaking)
		case state.Sprinting:
			m.setFlag(dataKeyFlags, dataFlagSprinting)
		case state.Breathing:
			m.setFlag(dataKeyFlags, dataFlagBreathing)
		case state.Invisible:
			m.setFlag(dataKeyFlags, dataFlagInvisible)
		case state.Immobile:
			m.setFlag(dataKeyFlags, dataFlagNoAI)
		case state.Swimming:
			m.setFlag(dataKeyFlags, dataFlagSwimming)
		case state.UsingItem:
			m.setFlag(dataKeyFlags, dataFlagUsingItem)
		case state.Named:
			m[dataKeyNameTag] = st.NameTag
		case state.EffectBearing:
			m[dataKeyPotionColour] = (int32(st.ParticleColour.A) << 24) | (int32(st.ParticleColour.R) << 16) | (int32(st.ParticleColour.G) << 8) | int32(st.ParticleColour.B)
			if st.Ambient {
				m[dataKeyPotionAmbient] = byte(1)
			} else {
				m[dataKeyPotionAmbient] = byte(0)
			}
		case state.OnFire:
			m.setFlag(dataKeyFlags, dataFlagOnFire)
		case state.Scale:
			m[dataKeyScale] = float32(st.Scale)
		}
	}
	s.writePacket(&packet.SetActorData{
		EntityRuntimeID: s.entityRuntimeID(e),
		EntityMetadata:  m,
	})
}

// OpenBlockContainer ...
func (s *Session) OpenBlockContainer(pos world.BlockPos) {
	s.closeCurrentContainer()

	b := s.c.World().Block(pos)
	container, ok := b.(block.Container)
	if ok {
		s.openNormalContainer(container, pos)
		return
	}
	// We hit a special kind of window like beacons, which are not actually opened server-side.
	nextID := s.nextWindowID()
	s.containerOpened.Store(true)
	s.openedWindow.Store(inventory.New(1, nil))
	s.openedPos.Store(pos)

	var containerType byte
	switch b.(type) {
	case block.Beacon:
		containerType = 13
	}
	s.writePacket(&packet.ContainerOpen{
		WindowID:                nextID,
		ContainerType:           containerType,
		ContainerPosition:       protocol.BlockPos{int32(pos[0]), int32(pos[1]), int32(pos[2])},
		ContainerEntityUniqueID: -1,
	})
}

// openNormalContainer opens a normal container that can hold items in it server-side.
func (s *Session) openNormalContainer(b block.Container, pos world.BlockPos) {
	b.AddViewer(s, s.c.World(), pos)

	nextID := s.nextWindowID()
	s.containerOpened.Store(true)
	s.openedWindow.Store(b.Inventory())
	s.openedPos.Store(pos)

	var containerType byte
	switch b.(type) {
	}

	s.writePacket(&packet.ContainerOpen{
		WindowID:                nextID,
		ContainerType:           containerType,
		ContainerPosition:       protocol.BlockPos{int32(pos[0]), int32(pos[1]), int32(pos[2])},
		ContainerEntityUniqueID: -1,
	})
	s.sendInv(b.Inventory(), uint32(nextID))
}

// ViewSlotChange ...
func (s *Session) ViewSlotChange(slot int, newItem item.Stack) {
	if !s.containerOpened.Load() {
		return
	}
	if s.inTransaction.Load() {
		// Don't send slot changes to the player itself.
		return
	}
	s.writePacket(&packet.InventorySlot{
		WindowID: s.openedWindowID.Load(),
		Slot:     uint32(slot),
		NewItem:  instanceFromItem(newItem),
	})
}

// ViewBlockAction ...
func (s *Session) ViewBlockAction(pos world.BlockPos, a blockAction.Action) {
	blockPos := protocol.BlockPos{int32(pos[0]), int32(pos[1]), int32(pos[2])}
	switch t := a.(type) {
	case blockAction.Open:
		s.writePacket(&packet.BlockEvent{
			Position:  blockPos,
			EventType: packet.BlockEventChangeChestState,
			EventData: 1,
		})
	case blockAction.Close:
		s.writePacket(&packet.BlockEvent{
			Position:  blockPos,
			EventType: packet.BlockEventChangeChestState,
		})
	case blockAction.StartCrack:
		s.writePacket(&packet.LevelEvent{
			EventType: packet.EventBlockStartBreak,
			Position:  vec64To32(pos.Vec3()),
			EventData: int32(65535 / (t.BreakTime.Seconds() * 20)),
		})
	case blockAction.StopCrack:
		s.writePacket(&packet.LevelEvent{
			EventType: packet.EventBlockStopBreak,
			Position:  vec64To32(pos.Vec3()),
			EventData: 0,
		})
	case blockAction.ContinueCrack:
		s.writePacket(&packet.LevelEvent{
			EventType: 3602,
			Position:  vec64To32(pos.Vec3()),
			EventData: int32(65535 / (t.BreakTime.Seconds() * 20)),
		})
	}
}

// ViewEmote ...
func (s *Session) ViewEmote(player world.Entity, emote uuid.UUID) {
	s.writePacket(&packet.Emote{
		EntityRuntimeID: s.entityRuntimeID(player),
		EmoteID:         emote.String(),
		Flags:           packet.EmoteFlagServerSide,
	})
}

// nextWindowID produces the next window ID for a new window. It is an int of 1-99.
func (s *Session) nextWindowID() byte {
	if s.openedWindowID.CAS(99, 1) {
		return 1
	}
	return byte(s.openedWindowID.Add(1))
}

// closeWindow closes the container window currently opened. If no window is open, closeWindow will do
// nothing.
func (s *Session) closeWindow() {
	if !s.containerOpened.CAS(true, false) {
		return
	}
	s.openedWindow.Store(inventory.New(1, nil))
	s.writePacket(&packet.ContainerClose{WindowID: byte(s.openedWindowID.Load())})
}

// blockRuntimeID returns the runtime ID of the block passed.
func (s *Session) blockRuntimeID(b world.Block) uint32 {
	id, _ := world.BlockRuntimeID(b)
	return id
}

// entityRuntimeID returns the runtime ID of the entity passed.
//noinspection GoCommentLeadingSpace
func (s *Session) entityRuntimeID(e world.Entity) uint64 {
	s.entityMutex.RLock()
	//lint:ignore S1005 Double assignment is done explicitly to prevent panics.
	id, _ := s.entityRuntimeIDs[e]
	s.entityMutex.RUnlock()
	return id
}

// entityFromRuntimeID attempts to return an entity by its runtime ID. False is returned if no entity with the
// ID could be found.
func (s *Session) entityFromRuntimeID(id uint64) (world.Entity, bool) {
	s.entityMutex.RLock()
	e, ok := s.entities[id]
	s.entityMutex.RUnlock()
	return e, ok
}

// Position ...
func (s *Session) Position() mgl64.Vec3 {
	return s.c.Position()
}

// vec32To64 converts a mgl32.Vec3 to a mgl64.Vec3.
func vec32To64(vec3 mgl32.Vec3) mgl64.Vec3 {
	return mgl64.Vec3{float64(vec3[0]), float64(vec3[1]), float64(vec3[2])}
}

// vec64To32 converts a mgl64.Vec3 to a mgl32.Vec3.
func vec64To32(vec3 mgl64.Vec3) mgl32.Vec3 {
	return mgl32.Vec3{float32(vec3[0]), float32(vec3[1]), float32(vec3[2])}
}

// abs ...
func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}
