package world

import (
	"context"
	"fmt"
	"github.com/df-mc/dragonfly/dragonfly/entity/physics"
	"github.com/df-mc/dragonfly/dragonfly/world/chunk"
	"github.com/df-mc/dragonfly/dragonfly/world/difficulty"
	"github.com/df-mc/dragonfly/dragonfly/world/gamemode"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/sirupsen/logrus"
	"go.uber.org/atomic"
	"math/rand"
	"sync"
	"time"
)

// World implements a Minecraft world. It manages all aspects of what players can see, such as blocks,
// entities and particles.
// World generally provides a synchronised state: All entities, blocks and players usually operate in this
// world, so World ensures that all its methods will always be safe for simultaneous calls.
type World struct {
	name atomic.String
	log  *logrus.Logger

	unixTime, currentTick, time atomic.Int64
	timeStopped                 atomic.Bool
	rdonly                      atomic.Bool

	lastPos   ChunkPos
	lastChunk *chunkData

	stopTick         context.Context
	cancelTick       context.CancelFunc
	stopCacheJanitor chan struct{}
	doneTicking      chan struct{}

	gameModeMu      sync.RWMutex
	defaultGameMode gamemode.GameMode

	difficultyMu sync.RWMutex
	difficulty   difficulty.Difficulty

	handlerMu sync.RWMutex
	handler   Handler

	providerMu sync.RWMutex
	prov       Provider

	genMu sync.RWMutex
	gen   Generator

	chunkMu sync.RWMutex
	// chunks holds a cache of chunks currently loaded. These chunks are cleared from this map after some time
	// of not being used.
	chunks map[ChunkPos]*chunkData

	ePosMu sync.Mutex
	// lastEntityPositions holds a map of the last ChunkPos that an Entity was in. These are tracked so that
	// a call to RemoveEntity can find the correct entity.
	lastEntityPositions map[Entity]ChunkPos

	r         *rand.Rand
	simDistSq int32

	randomTickSpeed atomic.Uint32

	updateMu sync.Mutex
	// blockUpdates is a map of tick time values indexed by the block position at which an update is
	// scheduled. If the current tick exceeds the tick value passed, the block update will be performed
	// and the entry will be removed from the map.
	blockUpdates             map[BlockPos]int64
	updatePositions          []BlockPos
	neighbourUpdatePositions []neighbourUpdate
	neighbourUpdatesSync     []neighbourUpdate

	toTick              []toTick
	blockEntitiesToTick []blockEntityToTick
	positionCache       []ChunkPos
	entitiesToTick      []TickerEntity
}

// New creates a new initialised world. The world may be used right away, but it will not be saved or loaded
// from files until it has been given a different provider than the default. (NoIOProvider)
// By default, the name of the world will be 'World'.
func New(log *logrus.Logger, simulationDistance int) *World {
	ctx, cancel := context.WithCancel(context.Background())
	w := &World{
		r:                   rand.New(rand.NewSource(time.Now().Unix())),
		blockUpdates:        map[BlockPos]int64{},
		lastEntityPositions: map[Entity]ChunkPos{},
		defaultGameMode:     gamemode.Survival{},
		difficulty:          difficulty.Normal{},
		prov:                NoIOProvider{},
		gen:                 NopGenerator{},
		handler:             NopHandler{},
		doneTicking:         make(chan struct{}),
		stopCacheJanitor:    make(chan struct{}),
		simDistSq:           int32(simulationDistance * simulationDistance),
		randomTickSpeed:     *atomic.NewUint32(3),
		unixTime:            *atomic.NewInt64(time.Now().Unix()),
		log:                 log,
		stopTick:            ctx,
		cancelTick:          cancel,
		name:                *atomic.NewString("World"),
	}
	w.initChunkCache()
	go w.startTicking()
	go w.chunkCacheJanitor()
	return w
}

// Name returns the display name of the world. Generally, this name is displayed at the top of the player list
// in the pause screen in-game.
// If a provider is set, the name will be updated according to the name that it provides.
func (w *World) Name() string {
	return w.name.Load()
}

// Block reads a block from the position passed. If a chunk is not yet loaded at that position, the chunk is
// loaded, or generated if it could not be found in the world save, and the block returned. Chunks will be
// loaded synchronously.
func (w *World) Block(pos BlockPos) Block {
	y := pos[1]
	if y > 255 || y < 0 {
		// Fast way out.
		return air()
	}
	chunkPos := ChunkPos{int32(pos[0] >> 4), int32(pos[2] >> 4)}
	c, err := w.chunk(chunkPos)
	if err != nil {
		w.log.Errorf("error getting block: %v", err)
		return air()
	}
	rid := c.RuntimeID(uint8(pos[0]), uint8(pos[1]), uint8(pos[2]), 0)
	c.Unlock()

	state := registeredStates[rid]
	if state.HasNBT() {
		if _, ok := state.(NBTer); ok {
			// The block was also a block entity, so we look it up in the block entity map.
			b, ok := c.e[pos]
			if ok {
				return b
			}
		}
	}
	return state
}

// blockInChunk reads a block from the world at the position passed. The block is assumed to be in the chunk
// passed, which is also assumed to be locked already or otherwise not yet accessible.
func (w *World) blockInChunk(c *chunkData, pos BlockPos) (Block, error) {
	if pos.OutOfBounds() {
		// Fast way out.
		return air(), nil
	}
	state := registeredStates[c.RuntimeID(uint8(pos[0]), uint8(pos[1]), uint8(pos[2]), 0)]

	if _, ok := state.(NBTer); ok {
		// The block was also a block entity, so we look it up in the block entity map.
		b, ok := c.e[pos]
		if ok {
			return b, nil
		}
	}
	return state, nil
}

// runtimeID gets the block runtime ID at a specific position in the world.
//lint:ignore U1000 Function is used using compiler directives.
//noinspection GoUnusedFunction
func runtimeID(w *World, pos BlockPos) uint32 {
	if pos[1] < 0 || pos[1] > 255 {
		// Fast way out.
		return 0
	}
	c, err := w.chunk(ChunkPos{int32(pos[0] >> 4), int32(pos[2] >> 4)})
	if err != nil {
		return 0
	}
	rid := c.RuntimeID(uint8(pos[0]), uint8(pos[1]), uint8(pos[2]), 0)
	c.Unlock()

	return rid
}

// highestLightBlocker gets the Y value of the highest fully light blocking block at the x and z values
// passed in the world.
//lint:ignore U1000 Function is used using compiler directives.
//noinspection GoUnusedFunction
func highestLightBlocker(w *World, x, z int) uint8 {
	c, err := w.chunk(ChunkPos{int32(x >> 4), int32(z >> 4)})
	if err != nil {
		return 0
	}
	v := c.HighestLightBlocker(uint8(x), uint8(z))
	c.Unlock()
	return v
}

// HighestBlock looks up the highest non-air block in the world at a specific x and z in the world. The y
// value of the highest block is returned, or 0 if no blocks were present in the column.
func (w *World) HighestBlock(x, z int) int {
	c, err := w.chunk(ChunkPos{int32(x >> 4), int32(z >> 4)})
	if err != nil {
		return 0
	}
	v := c.HighestBlock(uint8(x), uint8(z))
	c.Unlock()
	return int(v)
}

// SetBlock writes a block to the position passed. If a chunk is not yet loaded at that position, the chunk is
// first loaded or generated if it could not be found in the world save.
// SetBlock panics if the block passed has not yet been registered using RegisterBlock().
// Nil may be passed as the block to set the block to air.
// SetBlock should be avoided in situations where performance is critical when needing to set a lot of blocks
// to the world. BuildStructure may be used instead.
func (w *World) SetBlock(pos BlockPos, b Block) {
	y := pos[1]
	if y > 255 || y < 0 {
		// Fast way out.
		return
	}
	x, z := int32(pos[0]>>4), int32(pos[2]>>4)
	c, err := w.chunk(ChunkPos{x, z})
	if err != nil {
		return
	}
	var h int64
	if b != nil {
		h = int64(b.Hash())
	}
	runtimeID, ok := runtimeIDsHashes.Get(h)
	if !ok {
		w.log.Errorf("runtime ID of block state %+v not found", b)
		c.Unlock()
		return
	}
	c.SetRuntimeID(uint8(pos[0]), uint8(pos[1]), uint8(pos[2]), 0, uint32(runtimeID))

	var hasNBT bool
	if b != nil {
		hasNBT = b.HasNBT()
	}
	if hasNBT {
		if _, hasNBT := b.(NBTer); hasNBT {
			c.e[pos] = b
		}
	} else {
		delete(c.e, pos)
	}
	c.Unlock()
	for _, viewer := range c.v {
		viewer.ViewBlockUpdate(pos, b, 0)
	}
}

// setBlockInChunk sets a block in the chunk passed at a specific position. Unlike setBlock, setBlockInChunk
// does not send block updates to viewer.
func (w *World) setBlockInChunk(c *chunkData, pos BlockPos, b Block) error {
	runtimeID, ok := runtimeIDsHashes.Get(int64(b.Hash()))
	if !ok {
		return fmt.Errorf("runtime ID of block state %+v not found", b)
	}
	c.SetRuntimeID(uint8(pos[0]), uint8(pos[1]), uint8(pos[2]), 0, uint32(runtimeID))

	if _, hasNBT := b.(NBTer); hasNBT {
		c.e[pos] = b
	} else {
		delete(c.e, pos)
	}
	return nil
}

// breakParticle has its value set in the block_internal package.
var breakParticle func(b Block) Particle

// BreakBlock breaks a block at the position passed. Unlike when setting the block at that position to air,
// BreakBlock will also show particles and update blocks around the position.
func (w *World) BreakBlock(pos BlockPos) {
	old := w.Block(pos)
	w.SetBlock(pos, nil)
	w.AddParticle(pos.Vec3Centre(), breakParticle(old))
	if liq, ok := w.Liquid(pos); ok {
		// Move the liquid down a layer.
		w.SetLiquid(pos, liq)
	} else {
		w.doBlockUpdatesAround(pos)
	}
}

// BreakBlockWithoutParticles breaks a block at the position passed. Unlike when setting the block at that position to air,
// BreakBlockWithoutParticles will also update blocks around the position.
func (w *World) BreakBlockWithoutParticles(pos BlockPos) {
	w.SetBlock(pos, nil)
	if liq, ok := w.Liquid(pos); ok {
		// Move the liquid down a layer.
		w.SetLiquid(pos, liq)
	} else {
		w.doBlockUpdatesAround(pos)
	}
}

// PlaceBlock places a block at the position passed. Unlike when using SetBlock, PlaceBlock also schedules
// block updates around the position.
// If the block can displace liquids at the position placed, it will do so, and liquid source blocks will be
// put into the same block as the one passed.
func (w *World) PlaceBlock(pos BlockPos, b Block) {
	var liquid Liquid
	if displacer, ok := b.(LiquidDisplacer); ok {
		liq, ok := w.Liquid(pos)
		if ok && displacer.CanDisplace(liq) && liq.LiquidDepth() == 8 {
			liquid = liq
		}
	}
	w.SetBlock(pos, b)
	if liquid != nil {
		w.SetLiquid(pos, liquid)
		return
	}
	w.SetLiquid(pos, nil)
}

// BuildStructure builds a Structure passed at a specific position in the world. Unlike SetBlock, it takes a
// Structure implementation, which provides blocks to be placed at a specific location.
// BuildStructure is specifically tinkered to be able to process a large batch of chunks simultaneously and
// will do so within much less time than separate SetBlock calls would.
// The method operates on a per-chunk basis, setting all blocks within a single chunk part of the structure
// before moving on to the next chunk.
func (w *World) BuildStructure(pos BlockPos, s Structure) {
	dim := s.Dimensions()
	width, height, length := dim[0], dim[1], dim[2]
	maxX, maxZ := pos[0]+width, pos[2]+length

	for chunkX := pos[0] >> 4; chunkX < ((pos[0]+width)>>4)+1; chunkX++ {
		for chunkZ := pos[2] >> 4; chunkZ < ((pos[2]+length)>>4)+1; chunkZ++ {
			// We approach this on a per-chunk basis, so that we can keep only one chunk in memory at a time
			// while not needing to acquire a new chunk lock for every block. This also allows us not to send
			// block updates, but instead send a single chunk update once.

			chunkPos := ChunkPos{int32(chunkX), int32(chunkZ)}
			c, err := w.chunk(chunkPos)
			if err != nil {
				w.log.Errorf("error loading chunk for structure: %v", err)
			}
			f := func(x, y, z int) Block {
				if x>>4 == chunkX && z>>4 == chunkZ {
					b, _ := w.blockInChunk(c, BlockPos{x, y, z})
					return b
				}
				return w.Block(BlockPos{x, y, z})
			}
			baseX, baseZ := chunkX<<4, chunkZ<<4
			for localX := 0; localX < 16; localX++ {
				xOffset := baseX + localX
				if xOffset < pos[0] || xOffset >= maxX {
					continue
				}
				for localZ := 0; localZ < 16; localZ++ {
					zOffset := baseZ + localZ
					if zOffset < pos[2] || zOffset >= maxZ {
						continue
					}
					for y := 0; y < height; y++ {
						if y+pos[1] > 255 {
							// We've hit the height limit for blocks.
							break
						} else if y+pos[1] < 0 {
							// We've got a block below the minimum, but other blocks might still reach above
							// it, so don't break but continue.
							continue
						}
						placePos := BlockPos{xOffset, y + pos[1], zOffset}
						if b := s.At(xOffset-pos[0], y, zOffset-pos[2], f); b != nil {
							if err := w.setBlockInChunk(c, placePos, b); err != nil {
								w.log.Errorf("error setting block of structure: %v", err)
							}
						}
						if liq := s.AdditionalLiquidAt(xOffset-pos[0], y, zOffset-pos[2]); liq != nil {
							runtimeID, ok := BlockRuntimeID(liq)
							if !ok {
								w.log.Errorf("runtime ID of block state %+v not found", liq)
								continue
							}
							c.SetRuntimeID(uint8(xOffset), uint8(y+pos[1]), uint8(zOffset), 1, runtimeID)
						} else {
							c.SetRuntimeID(uint8(xOffset), uint8(y+pos[1]), uint8(zOffset), 1, 0)
						}
					}
				}
			}
			// After setting all blocks of the structure within a single chunk, we show the new chunk to all
			// viewers once, and unlock it.
			for _, viewer := range c.v {
				viewer.ViewChunk(chunkPos, c.Chunk, c.e)
			}
			c.Unlock()
		}
	}
}

// Liquid attempts to return any liquid block at the position passed. This liquid may be in the foreground or
// in any other layer.
// If found, the liquid is returned. If not, the bool returned is false and the liquid is nil.
func (w *World) Liquid(pos BlockPos) (Liquid, bool) {
	if pos.OutOfBounds() {
		// Fast way out.
		return nil, false
	}
	c, err := w.chunk(chunkPosFromBlockPos(pos))
	if err != nil {
		w.log.Errorf("failed getting liquid: error getting chunk at position %v: %w", chunkPosFromBlockPos(pos), err)
		return nil, false
	}
	x, y, z := uint8(pos[0]), uint8(pos[1]), uint8(pos[2])

	id := c.RuntimeID(x, y, z, 0)
	b, ok := blockByRuntimeID(id)
	if !ok {
		w.log.Errorf("failed getting liquid: cannot get block by runtime ID %v", id)
		c.Unlock()
		return nil, false
	}
	if liq, ok := b.(Liquid); ok {
		c.Unlock()
		return liq, true
	}

	id = c.RuntimeID(x, y, z, 1)
	b, ok = blockByRuntimeID(id)
	c.Unlock()
	if !ok {
		w.log.Errorf("failed getting liquid: cannot get block by runtime ID %v", id)
		return nil, false
	}
	if liq, ok := b.(Liquid); ok {
		return liq, true
	}
	return nil, false
}

// SetLiquid sets the liquid at a specific position in the world. Unlike SetBlock, SetLiquid will not
// overwrite any existing blocks. It will instead be in the same position as a block currently there, unless
// there already is a liquid at that position, in which case it will be overwritten.
// If nil is passed for the liquid, any liquid currently present will be removed.
func (w *World) SetLiquid(pos BlockPos, b Liquid) {
	if pos.OutOfBounds() {
		// Fast way out.
		return
	}
	chunkPos := chunkPosFromBlockPos(pos)
	c, err := w.chunk(chunkPos)
	if err != nil {
		w.log.Errorf("failed setting liquid: error getting chunk at position %v: %w", chunkPosFromBlockPos(pos), err)
		return
	}
	if b == nil {
		w.removeLiquids(c, pos)
		c.Unlock()
		w.doBlockUpdatesAround(pos)
		return
	}
	x, y, z := uint8(pos[0]), uint8(pos[1]), uint8(pos[2])
	if !replaceable(w, c, pos, b) {
		current, err := w.blockInChunk(c, pos)
		if err != nil {
			c.Unlock()
			w.log.Errorf("failed setting liquid: error getting block at position %v: %w", chunkPosFromBlockPos(pos), err)
			return
		}
		if displacer, ok := current.(LiquidDisplacer); !ok || !displacer.CanDisplace(b) {
			c.Unlock()
			return
		}
	}
	runtimeID, ok := BlockRuntimeID(b)
	if !ok {
		c.Unlock()
		w.log.Errorf("failed setting liquid: runtime ID of block state %+v not found", b)
		return
	}
	if w.removeLiquids(c, pos) {
		c.SetRuntimeID(x, y, z, 0, runtimeID)
		for _, v := range c.v {
			v.ViewBlockUpdate(pos, b, 0)
		}
	} else {
		c.SetRuntimeID(x, y, z, 1, runtimeID)
		for _, v := range c.v {
			v.ViewBlockUpdate(pos, b, 1)
		}
	}
	c.Unlock()

	w.doBlockUpdatesAround(pos)
}

// removeLiquids removes any liquid blocks that may be present at a specific block position in the chunk
// passed.
// The bool returned specifies if no blocks were left on the foreground layer.
func (w *World) removeLiquids(c *chunkData, pos BlockPos) bool {
	x, y, z := uint8(pos[0]), uint8(pos[1]), uint8(pos[2])

	noneLeft := false
	if noLeft, changed := w.removeLiquidOnLayer(c.Chunk, x, y, z, 0); noLeft {
		if changed {
			for _, v := range c.v {
				v.ViewBlockUpdate(pos, air(), 0)
			}
		}
		noneLeft = true
	}
	if _, changed := w.removeLiquidOnLayer(c.Chunk, x, y, z, 1); changed {
		for _, v := range c.v {
			v.ViewBlockUpdate(pos, air(), 1)
		}
	}
	return noneLeft
}

// removeLiquidOnLayer removes a liquid block from a specific layer in the chunk passed, returning true if
// successful.
func (w *World) removeLiquidOnLayer(c *chunk.Chunk, x, y, z, layer uint8) (bool, bool) {
	id := c.RuntimeID(x, y, z, layer)

	b, ok := blockByRuntimeID(id)
	if !ok {
		w.log.Errorf("failed removing liquids: cannot get block by runtime ID %v", id)
		return false, false
	}
	if _, ok := b.(Liquid); ok {
		c.SetRuntimeID(x, y, z, layer, 0)
		return true, true
	}
	return id == 0, false
}

// additionalLiquid checks if the block at a position has additional liquid on another layer and returns the
// liquid if so.
func (w *World) additionalLiquid(pos BlockPos) (Liquid, bool) {
	if pos.OutOfBounds() {
		// Fast way out.
		return nil, false
	}
	c, err := w.chunk(chunkPosFromBlockPos(pos))
	if err != nil {
		w.log.Errorf("failed getting liquid: error getting chunk at position %v: %w", chunkPosFromBlockPos(pos), err)
		return nil, false
	}
	id := c.RuntimeID(uint8(pos[0]), uint8(pos[1]), uint8(pos[2]), 1)
	c.Unlock()
	b, ok := blockByRuntimeID(id)
	if !ok {
		w.log.Errorf("failed getting liquid: cannot get block by runtime ID %v", id)
		return nil, false
	}
	liq, ok := b.(Liquid)
	return liq, ok
}

// Light returns the light level at the position passed. This is the highest of the sky and block light.
// The light value returned is a value in the range 0-15, where 0 means there is no light present, whereas
// 15 means the block is fully lit.
func (w *World) Light(pos BlockPos) uint8 {
	if pos[1] > 255 {
		// Above the rest of the world, so full sky light.
		return 15
	}
	if pos[1] < 0 {
		// Fast way out.
		return 0
	}
	c, err := w.chunk(chunkPosFromBlockPos(pos))
	if err != nil {
		return 0
	}
	l := c.Light(uint8(pos[0]), uint8(pos[1]), uint8(pos[2]))
	c.Unlock()

	return l
}

// SkyLight returns the sky light level at the position passed. This light level is not influenced by blocks
// that emit light, such as torches or glowstone. The light value, similarly to Light, is a value in the
// range 0-15, where 0 means no light is present.
func (w *World) SkyLight(pos BlockPos) uint8 {
	if pos[1] > 255 {
		// Above the rest of the world, so full sky light.
		return 15
	}
	if pos[1] < 0 {
		// Fast way out.
		return 0
	}
	c, err := w.chunk(chunkPosFromBlockPos(pos))
	if err != nil {
		return 0
	}
	l := c.SkyLight(uint8(pos[0]), uint8(pos[1]), uint8(pos[2]))
	c.Unlock()

	return l
}

// Time returns the current time of the world. The time is incremented every 1/20th of a second, unless
// World.StopTime() is called.
func (w *World) Time() int {
	return int(w.time.Load())
}

// SetTime sets the new time of the world. SetTime will always work, regardless of whether the time is stopped
// or not.
func (w *World) SetTime(new int) {
	w.time.Store(int64(new))
	for _, viewer := range w.allViewers() {
		viewer.ViewTime(new)
	}
}

// StopTime stops the time in the world. When called, the time will no longer cycle and the world will remain
// at the time when StopTime is called. The time may be restarted by calling World.StartTime().
// StopTime will not do anything if the time is already stopped.
func (w *World) StopTime() {
	w.timeStopped.Store(true)
}

// StartTime restarts the time in the world. When called, the time will start cycling again and the day/night
// cycle will continue. The time may be stopped again by calling World.StopTime().
// StartTime will not do anything if the time is already started.
func (w *World) StartTime() {
	w.timeStopped.Store(false)
}

// AddParticle spawns a particle at a given position in the world. Viewers that are viewing the chunk will be
// shown the particle.
func (w *World) AddParticle(pos mgl64.Vec3, p Particle) {
	p.Spawn(w, pos)
	for _, viewer := range w.Viewers(pos) {
		viewer.ViewParticle(pos, p)
	}
}

// PlaySound plays a sound at a specific position in the world. Viewers of that position will be able to hear
// the sound if they're close enough.
func (w *World) PlaySound(pos mgl64.Vec3, s Sound) {
	for _, viewer := range w.Viewers(pos) {
		viewer.ViewSound(pos, s)
	}
}

// entityWorlds holds a list of all entities added to a world. It may be used to lookup the world that an
// entity is currently in.
var entityWorlds = map[Entity]*World{}
var worldsMu sync.RWMutex

// AddEntity adds an entity to the world at the position that the entity has. The entity will be visible to
// all viewers of the world that have the chunk of the entity loaded.
// If the chunk that the entity is in is not yet loaded, it will first be loaded.
// If the entity passed to AddEntity is currently in a world, it is first removed from that world.
func (w *World) AddEntity(e Entity) {
	if e.World() != nil {
		e.World().RemoveEntity(e)
	}
	worldsMu.Lock()
	entityWorlds[e] = w
	worldsMu.Unlock()

	chunkPos := chunkPosFromVec3(e.Position())
	c, err := w.chunk(chunkPos)
	if err != nil {
		w.log.Errorf("error loading chunk to add entity: %v", err)
	}
	viewers := c.v
	c.entities = append(c.entities, e)
	c.Unlock()

	for _, viewer := range viewers {
		// We show the entity to all viewers currently in the chunk that the entity is spawned in.
		showEntity(e, viewer)
	}
}

// RemoveEntity removes an entity from the world that is currently present in it. Any viewers of the entity
// will no longer be able to see it.
// RemoveEntity operates assuming the position of the entity is the same as where it is currently in the
// world. If it can not find it there, it will loop through all entities and try to find it.
// RemoveEntity assumes the entity is currently loaded and in a loaded chunk. If not, the function will not do
// anything.
func (w *World) RemoveEntity(e Entity) {
	w.ePosMu.Lock()
	chunkPos, found := w.lastEntityPositions[e]
	w.ePosMu.Unlock()
	if !found {
		chunkPos = chunkPosFromVec3(e.Position())
	} else {
		delete(w.lastEntityPositions, e)
	}

	worldsMu.Lock()
	delete(entityWorlds, e)
	worldsMu.Unlock()

	c, ok := w.chunkFromCache(chunkPos)
	if !ok {
		// The chunk wasn't loaded, so we can't remove any entity from the chunk.
		return
	}
	c.Lock()
	n := make([]Entity, 0, len(c.entities))
	for _, entity := range c.entities {
		if entity != e {
			n = append(n, entity)
			continue
		}
	}
	c.entities = n
	for _, viewer := range c.v {
		viewer.HideEntity(e)
	}
	c.Unlock()
}

// EntitiesWithin does a lookup through the entities in the chunks touched by the AABB passed, returning all
// those which are contained within the AABB when it comes to their position.
func (w *World) EntitiesWithin(aabb physics.AABB) []Entity {
	// Make an estimate of 16 entities on average.
	m := make([]Entity, 0, 16)

	// We expand it by 3 blocks in all horizontal directions to account for entities that may be in
	// neighbouring chunks while having a bounding box that extends into the current one.
	minPos, maxPos := chunkPosFromVec3(aabb.Min()), chunkPosFromVec3(aabb.Max())

	for x := minPos[0]; x <= maxPos[0]; x++ {
		for z := minPos[1]; z <= maxPos[1]; z++ {
			c, ok := w.chunkFromCache(ChunkPos{x, z})
			if !ok {
				// The chunk wasn't loaded, so there are no entities here.
				continue
			}
			c.Lock()
			for _, entity := range c.entities {
				if aabb.Vec3Within(entity.Position()) {
					// The entity position was within the AABB, so we add it to the slice to return.
					m = append(m, entity)
				}
			}
			c.Unlock()
		}
	}
	return m
}

// OfEntity attempts to return a world that an entity is currently in. If the entity was not currently added
// to a world, the world returned is nil and the bool returned is false.
func OfEntity(e Entity) (*World, bool) {
	worldsMu.RLock()
	w, ok := entityWorlds[e]
	worldsMu.RUnlock()
	return w, ok
}

// Spawn returns the spawn of the world. Every new player will by default spawn on this position in the world
// when joining.
func (w *World) Spawn() BlockPos {
	return w.provider().WorldSpawn()
}

// SetSpawn sets the spawn of the world to a different position. The player will be spawned in the center of
// this position when newly joining.
func (w *World) SetSpawn(pos BlockPos) {
	w.provider().SetWorldSpawn(pos)
}

// DefaultGameMode returns the default game mode of the world. When players join, they are given this game
// mode.
// The default game mode may be changed using SetDefaultGameMode().
func (w *World) DefaultGameMode() gamemode.GameMode {
	w.gameModeMu.RLock()
	defer w.gameModeMu.RUnlock()
	return w.defaultGameMode
}

// SetDefaultGameMode changes the default game mode of the world. When players join, they are then given that
// game mode.
func (w *World) SetDefaultGameMode(mode gamemode.GameMode) {
	w.gameModeMu.Lock()
	defer w.gameModeMu.Unlock()
	w.defaultGameMode = mode
}

// Difficulty returns the difficulty of the world. Properties of mobs in the world and the player's hunger
// will depend on this difficulty.
func (w *World) Difficulty() difficulty.Difficulty {
	w.difficultyMu.RLock()
	defer w.difficultyMu.RUnlock()
	return w.difficulty
}

// SetDifficulty changes the difficulty of a world.
func (w *World) SetDifficulty(d difficulty.Difficulty) {
	w.difficultyMu.Lock()
	defer w.difficultyMu.Unlock()
	w.difficulty = d
}

// SetRandomTickSpeed sets the random tick speed of blocks. By default, each sub chunk has 3 blocks randomly
// ticked per sub chunk, so the default value is 3. Setting this value to 0 will stop random ticking
// altogether, while setting it higher results in faster ticking.
func (w *World) SetRandomTickSpeed(v int) {
	w.randomTickSpeed.Store(uint32(v))
}

// ScheduleBlockUpdate schedules a block update at the position passed after a specific delay. If the block at
// that position does not handle block updates, nothing will happen.
func (w *World) ScheduleBlockUpdate(pos BlockPos, delay time.Duration) {
	if pos.OutOfBounds() {
		return
	}
	w.updateMu.Lock()
	if _, exists := w.blockUpdates[pos]; exists {
		w.updateMu.Unlock()
		return
	}
	w.blockUpdates[pos] = w.currentTick.Load() + delay.Nanoseconds()/int64(time.Second/20)
	w.updateMu.Unlock()
}

// doBlockUpdatesAround schedules block updates directly around and on the position passed.
func (w *World) doBlockUpdatesAround(pos BlockPos) {
	if pos.OutOfBounds() {
		return
	}

	changed := pos

	w.updateMu.Lock()
	w.updateNeighbour(pos, changed)
	pos.Neighbours(func(pos BlockPos) {
		w.updateNeighbour(pos, changed)
	})
	w.updateMu.Unlock()
}

// neighbourUpdate represents a position that needs to be updated because of a neighbour that changed.
type neighbourUpdate struct {
	pos, neighbour BlockPos
}

// updateNeighbour ticks the position passed as a result of the neighbour passed being updated.
func (w *World) updateNeighbour(pos, changedNeighbour BlockPos) {
	w.neighbourUpdatePositions = append(w.neighbourUpdatePositions, neighbourUpdate{pos: pos, neighbour: changedNeighbour})

}

// Provider changes the provider of the world to the provider passed. If nil is passed, the NoIOProvider
// will be set, which does not read or write any data.
func (w *World) Provider(p Provider) {
	w.providerMu.Lock()
	defer w.providerMu.Unlock()

	if p == nil {
		p = NoIOProvider{}
	}
	w.prov = p
	w.name.Store(p.WorldName())
	w.gameModeMu.Lock()
	w.defaultGameMode = p.LoadDefaultGameMode()
	w.gameModeMu.Unlock()
	w.difficultyMu.Lock()
	w.difficulty = p.LoadDifficulty()
	w.difficultyMu.Unlock()
	w.time.Store(p.LoadTime())
	w.timeStopped.Store(!p.LoadTimeCycle())
	w.initChunkCache()
}

// ReadOnly makes the world read only. Chunks will no longer be saved to disk, just like entities and data
// in the level.dat.
func (w *World) ReadOnly() {
	w.rdonly.Store(true)
}

// Generator changes the generator of the world to the one passed. If nil is passed, the generator is set to
// the default, NopGenerator.
func (w *World) Generator(g Generator) {
	w.genMu.Lock()
	defer w.genMu.Unlock()

	if g == nil {
		g = NopGenerator{}
	}
	w.gen = g
}

// Handle changes the current Handler of the world. As a result, events called by the world will call
// handlers of the Handler passed.
// Handle sets the world's Handler to NopHandler if nil is passed.
func (w *World) Handle(h Handler) {
	w.handlerMu.Lock()
	defer w.handlerMu.Unlock()

	if h == nil {
		h = NopHandler{}
	}
	w.handler = h
}

// Viewers returns a list of all viewers viewing the position passed. A viewer will be assumed to be watching
// if the position is within one of the chunks that the viewer is watching.
func (w *World) Viewers(pos mgl64.Vec3) []Viewer {
	c, ok := w.chunkFromCache(chunkPosFromVec3(pos))
	if !ok {
		return nil
	}
	c.Lock()
	viewers := make([]Viewer, len(c.v))
	copy(viewers, c.v)
	c.Unlock()
	return viewers
}

// Close closes the world and saves all chunks currently loaded.
func (w *World) Close() error {
	w.cancelTick()
	<-w.doneTicking

	w.log.Debug("Saving chunks in memory to disk...")

	w.chunkMu.Lock()
	chunksToSave := make(map[ChunkPos]*chunkData, len(w.chunks))
	for pos, c := range w.chunks {
		// We delete all chunks from the cache and save them to the provider.
		delete(w.chunks, pos)
		chunksToSave[pos] = c
	}
	w.chunkMu.Unlock()

	for pos, c := range chunksToSave {
		w.saveChunk(pos, c)
	}

	if !w.rdonly.Load() {
		w.log.Debug("Updating level.dat values...")
		w.provider().SaveTime(w.time.Load())
		w.provider().SaveTimeCycle(!w.timeStopped.Load())

		w.gameModeMu.RLock()
		w.provider().SaveDefaultGameMode(w.defaultGameMode)
		w.gameModeMu.RUnlock()
		w.difficultyMu.RLock()
		w.provider().SaveDifficulty(w.difficulty)
		w.difficultyMu.RUnlock()
	}

	w.log.Debug("Closing provider...")
	if err := w.provider().Close(); err != nil {
		w.log.Errorf("error closing world provider: %v", err)
	}
	w.Handle(NopHandler{})
	return nil
}

// startTicking starts ticking the world, updating all entities, blocks and other features such as the time of
// the world, as required.
func (w *World) startTicking() {
	ticker := time.NewTicker(time.Second / 20)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.unixTime.Store(time.Now().Unix())
			w.tick()
		case <-w.stopTick.Done():
			// The world was closed, so we should stop ticking.
			w.doneTicking <- struct{}{}
			return
		}
	}
}

// tick ticks the world and updates the time, blocks and entities that require updates.
func (w *World) tick() {
	viewers := w.allViewers()
	if len(viewers) == 0 {
		return
	}

	tick := w.currentTick.Add(1)

	if !w.timeStopped.Load() {
		w.time.Add(1)
	}
	if tick%20 == 0 {
		for _, viewer := range viewers {
			viewer.ViewTime(int(w.time.Load()))
		}
	}
	w.tickEntities(tick)
	w.tickRandomBlocks(viewers, tick)
	w.tickScheduledBlocks(tick)
}

// tickScheduledBlocks executes scheduled block ticks in chunks that are still loaded at the time of
// execution.
func (w *World) tickScheduledBlocks(tick int64) {
	w.updateMu.Lock()
	for pos, scheduledTick := range w.blockUpdates {
		if scheduledTick <= tick {
			w.updatePositions = append(w.updatePositions, pos)
			delete(w.blockUpdates, pos)
		}
	}
	w.neighbourUpdatesSync = append(w.neighbourUpdatesSync, w.neighbourUpdatePositions...)
	w.neighbourUpdatePositions = w.neighbourUpdatePositions[:0]
	w.updateMu.Unlock()

	for _, pos := range w.updatePositions {
		if ticker, ok := w.Block(pos).(ScheduledTicker); ok {
			ticker.ScheduledTick(pos, w)
		}
		if liquid, ok := w.additionalLiquid(pos); ok {
			if ticker, ok := liquid.(ScheduledTicker); ok {
				ticker.ScheduledTick(pos, w)
			}
		}
	}
	for _, update := range w.neighbourUpdatesSync {
		pos, changedNeighbour := update.pos, update.neighbour
		if ticker, ok := w.Block(pos).(NeighbourUpdateTicker); ok {
			ticker.NeighbourUpdateTick(pos, changedNeighbour, w)
		}
		if liquid, ok := w.additionalLiquid(pos); ok {
			if ticker, ok := liquid.(NeighbourUpdateTicker); ok {
				ticker.NeighbourUpdateTick(pos, changedNeighbour, w)
			}
		}
	}

	w.updatePositions = w.updatePositions[:0]
	w.neighbourUpdatesSync = w.neighbourUpdatesSync[:0]
}

// toTick is a struct used to keep track of blocks that need to be ticked upon a random tick.
type toTick struct {
	b   RandomTicker
	pos BlockPos
}

// blockEntityToTick is a struct used to keep track of block entities that need to be ticked upon a normal
// world tick.
type blockEntityToTick struct {
	b   TickerBlock
	pos BlockPos
}

// tickRandomBlocks executes random block ticks in each sub chunk in the world that has at least one viewer
// registered from the viewers passed.
func (w *World) tickRandomBlocks(viewers []Viewer, tick int64) {
	if w.simDistSq == 0 {
		// NOP if the simulation distance is 0.
		return
	}
	tickSpeed := w.randomTickSpeed.Load()

	for _, viewer := range viewers {
		pos := viewer.Position()
		w.positionCache = append(w.positionCache, ChunkPos{
			// Technically we could obtain the wrong chunk position here due to truncating, but this
			// inaccuracy doesn't matter and it allows us to cut a corner.
			int32(pos[0]) >> 4,
			int32(pos[2]) >> 4,
		})
	}

	w.chunkMu.RLock()
	for pos := range w.chunks {
		c := w.chunks[pos]

		withinSimDist := false
		for _, chunkPos := range w.positionCache {
			xDiff, zDiff := chunkPos[0]-pos[0], chunkPos[1]-pos[1]
			if (xDiff*xDiff)+(zDiff*zDiff) <= w.simDistSq {
				// The chunk was within the simulation distance of at least one viewer, so we can proceed to
				// ticking the block.
				withinSimDist = true
				break
			}
		}
		if !withinSimDist {
			// No viewers in this chunk that are within the simulation distance, so proceed to the next.
			continue
		}
		c.Lock()
		for pos, b := range c.e {
			if ticker, ok := b.(TickerBlock); ok {
				w.blockEntitiesToTick = append(w.blockEntitiesToTick, blockEntityToTick{
					b:   ticker,
					pos: pos,
				})
			}
		}

		subChunks := c.Sub()
		// In total we generate 3 random blocks per sub chunk.
		for j := uint32(0); j < tickSpeed; j++ {
			// We generate 3 random uint64s. Out of a single uint64, we can pull 16 uint4s, which means we can
			// obtain a total of 16 coordinates on one axis from one uint64. One for each sub chunk.
			ra, rb, rc := int(w.r.Uint64()), int(w.r.Uint64()), int(w.r.Uint64())
			for i := 0; i < 64; i += 4 {
				sub := subChunks[i>>2]
				if sub == nil {
					// No sub chunk present, so skip it right away.
					continue
				}
				layers := sub.Layers()
				if len(layers) == 0 {
					// No layers present, so skip it right away.
					continue
				}
				x, y, z := (ra>>i)&0xf, (rb>>i)&0xf, (rc>>i)&0xf

				// Generally we would want to make sure the block has its block entities, but provided blocks
				// with block entities are generally ticked already, we are safe to assume that blocks
				// implementing the RandomTicker don't rely on additional block entity data.
				rid := layers[0].RuntimeID(uint8(x), uint8(y), uint8(z))
				if rid == 0 {
					// The block was air, take the fast route out.
					continue
				}

				if randomTicker, ok := registeredStates[rid].(RandomTicker); ok {
					w.toTick = append(w.toTick, toTick{b: randomTicker, pos: BlockPos{int(pos[0]<<4) + x, y + i<<2, int(pos[1]<<4) + z}})
				}
			}
		}
		c.Unlock()
	}
	w.chunkMu.RUnlock()

	for _, a := range w.toTick {
		a.b.RandomTick(a.pos, w, w.r)
	}
	for _, b := range w.blockEntitiesToTick {
		b.b.Tick(tick, b.pos, w)
	}
	w.toTick = w.toTick[:0]
	w.blockEntitiesToTick = w.blockEntitiesToTick[:0]
	w.positionCache = w.positionCache[:0]
}

// tickEntities ticks all entities in the world, making sure they are still located in the correct chunks and
// updating where necessary.
func (w *World) tickEntities(tick int64) {
	type entityToMove struct {
		e             Entity
		after         *chunkData
		viewersBefore []Viewer
	}
	var entitiesToMove []entityToMove

	w.chunkMu.RLock()
	// We first iterate over all chunks to see if entities move out of them. We make sure not to lock two
	// chunks at the same time.
	for chunkPos, c := range w.chunks {
		c.Lock()
		chunkEntities := make([]Entity, 0, len(c.entities))
		for _, entity := range c.entities {
			if ticker, ok := entity.(TickerEntity); ok {
				w.entitiesToTick = append(w.entitiesToTick, ticker)
			}

			// The entity was stored using an outdated chunk position. We update it and make sure it is ready
			// for viewers to view it.
			newChunkPos := chunkPosFromVec3(entity.Position())
			if newChunkPos != chunkPos {
				newC, ok := w.chunks[newChunkPos]
				if !ok {
					continue
				}
				w.ePosMu.Lock()
				w.lastEntityPositions[entity] = newChunkPos
				w.ePosMu.Unlock()
				entitiesToMove = append(entitiesToMove, entityToMove{e: entity, viewersBefore: append([]Viewer(nil), c.v...), after: newC})
				continue
			}
			chunkEntities = append(chunkEntities, entity)
		}
		c.entities = chunkEntities
		c.Unlock()
	}
	w.chunkMu.RUnlock()

	for _, move := range entitiesToMove {
		move.after.Lock()
		move.after.entities = append(move.after.entities, move.e)
		viewersAfter := move.after.v
		move.after.Unlock()

		for _, viewer := range move.viewersBefore {
			if !w.hasViewer(viewer, viewersAfter) {
				// First we hide the entity from all viewers that were previously viewing it, but no
				// longer are.
				viewer.HideEntity(move.e)
			}
		}
		for _, viewer := range viewersAfter {
			if !w.hasViewer(viewer, move.viewersBefore) {
				// Then we show the entity to all viewers that are now viewing the entity in the new
				// chunk.
				showEntity(move.e, viewer)
			}
		}
	}
	for _, ticker := range w.entitiesToTick {
		if _, ok := OfEntity(ticker.(Entity)); !ok {
			continue
		}
		// We gather entities to tick and tick them later, so that the lock on the entity mutex is no longer
		// active.
		ticker.Tick(tick)
	}
	w.entitiesToTick = w.entitiesToTick[:0]
}

// addViewer adds a viewer to the world at a given position. Any events that happen in the chunk at that
// position, such as block changes, entity changes etc., will be sent to the viewer.
func (w *World) addViewer(c *chunkData, viewer Viewer) {
	c.v = append(c.v, viewer)
	// After adding the viewer to the chunk, we also need to send all entities currently in the chunk that the
	// viewer is added to.
	entities := c.entities
	c.Unlock()
	for _, entity := range entities {
		showEntity(entity, viewer)
	}
	viewer.ViewTime(w.Time())
}

// removeViewer removes a viewer from the world at a given position. All entities will be hidden from the
// viewer and no more calls will be made when events in the chunk happen.
func (w *World) removeViewer(pos ChunkPos, viewer Viewer) {
	c, ok := w.chunkFromCache(pos)
	if !ok {
		return
	}
	c.Lock()
	n := make([]Viewer, 0, len(c.v))
	for _, v := range c.v {
		if v != viewer {
			// Add all viewers but the one to remove to the new viewers slice.
			n = append(n, v)
		}
	}
	c.v = n
	// After removing the viewer from the chunk, we also need to hide all entities from the viewer.
	for _, entity := range c.entities {
		viewer.HideEntity(entity)
	}
	c.Unlock()
}

// hasViewer checks if a chunk at a particular chunk position has the viewer passed. If so, true is returned.
func (w *World) hasViewer(viewer Viewer, viewers []Viewer) bool {
	for _, v := range viewers {
		if v == viewer {
			return true
		}
	}
	return false
}

// allViewers returns a list of all viewers of the world, regardless of where in the world they are viewing.
func (w *World) allViewers() []Viewer {
	var v []Viewer
	found := make(map[Viewer]struct{})

	w.chunkMu.RLock()
	for _, c := range w.chunks {
		c.Lock()
		for _, viewer := range c.v {
			if _, ok := found[viewer]; ok {
				// We've already found this viewer in another chunk. Don't add it again.
				continue
			}
			found[viewer] = struct{}{}
			v = append(v, viewer)
		}
		c.Unlock()
	}
	w.chunkMu.RUnlock()
	return v
}

// provider returns the provider of the world. It should always be used, rather than direct field access, in
// order to provide synchronisation safety.
func (w *World) provider() Provider {
	w.providerMu.RLock()
	provider := w.prov
	w.providerMu.RUnlock()
	return provider
}

// Handler returns the Handler of the world. It should always be used, rather than direct field access, in
// order to provide synchronisation safety.
func (w *World) Handler() Handler {
	w.handlerMu.RLock()
	handler := w.handler
	w.handlerMu.RUnlock()
	return handler
}

// generator returns the generator of the world. It should always be used, rather than direct field access, in
// order to provide synchronisation safety.
func (w *World) generator() Generator {
	w.genMu.RLock()
	generator := w.gen
	w.genMu.RUnlock()
	return generator
}

// chunkFromCache attempts to fetch a chunk at the chunk position passed from the cache. If not found, the
// chunk returned is nil and false is returned.
func (w *World) chunkFromCache(pos ChunkPos) (*chunkData, bool) {
	w.chunkMu.RLock()
	c, ok := w.chunks[pos]
	w.chunkMu.RUnlock()
	return c, ok
}

// showEntity shows an entity to a viewer of the world. It makes sure everything of the entity, including the
// items held, is shown.
func showEntity(e Entity, viewer Viewer) {
	viewer.ViewEntitySkin(e)
	viewer.ViewEntity(e)
	viewer.ViewEntityState(e, e.State())
	viewer.ViewEntityItems(e)
	viewer.ViewEntityArmour(e)
}

// chunk reads a chunk from the position passed. If a chunk at that position is not yet loaded, the chunk is
// loaded from the provider, or generated if it did not yet exist. Both of these actions are done
// synchronously.
// An error is returned if the chunk could not be loaded successfully.
// chunk locks the chunk returned, meaning that any call to chunk made at the same time has to wait until the
// user calls Chunk.Unlock() on the chunk returned.
func (w *World) chunk(pos ChunkPos) (*chunkData, error) {
	var err error

	w.chunkMu.Lock()
	if pos == w.lastPos && w.lastChunk != nil {
		c := w.lastChunk
		w.chunkMu.Unlock()
		c.Lock()
		return c, nil
	}
	c, ok := w.chunks[pos]
	if !ok {
		c, err = w.loadChunk(pos)
		if err != nil {
			w.chunkMu.Unlock()
			return nil, err
		}
		w.chunks[pos] = c
		w.calculateLight(c.Chunk, pos)
	}
	w.lastChunk, w.lastPos = c, pos
	w.chunkMu.Unlock()

	c.Lock()
	return c, nil
}

// setChunk sets the chunk.Chunk passed at a specific ChunkPos without replacing any entities at that
// position.
//lint:ignore U1000 This method is explicitly present to be used using compiler directives.
func (w *World) setChunk(pos ChunkPos, c *chunk.Chunk) {
	w.chunkMu.Lock()
	defer w.chunkMu.Unlock()

	data, ok := w.chunks[pos]
	if ok {
		data.Chunk = c
	} else {
		data = newChunkData(c)
		w.chunks[pos] = data
	}
	blockNBT := make([]map[string]interface{}, 0, len(c.BlockNBT()))
	for pos, e := range c.BlockNBT() {
		e["x"], e["y"], e["z"] = int32(pos[0]), int32(pos[1]), int32(pos[2])
		blockNBT = append(blockNBT, e)
	}
	w.loadIntoBlocks(data, blockNBT)
}

// loadChunk attempts to load a chunk from the provider, or generates a chunk if one doesn't currently exist.
func (w *World) loadChunk(pos ChunkPos) (*chunkData, error) {
	c, found, err := w.provider().LoadChunk(pos)

	if err != nil {
		return nil, fmt.Errorf("error loading chunk %v: %w", pos, err)
	}
	if !found {
		// The provider doesn't have a chunk saved at this position, so we generate a new one.
		c = chunk.New()
		w.generator().GenerateChunk(pos, c)
		return newChunkData(c), nil
	}
	data := newChunkData(c)
	entities, err := w.provider().LoadEntities(pos)
	if err != nil {
		return nil, fmt.Errorf("error loading entities of chunk %v: %w", pos, err)
	}
	data.entities = entities
	blockEntities, err := w.provider().LoadBlockNBT(pos)
	if err != nil {
		return nil, fmt.Errorf("error loading block entities of chunk %v: %w", pos, err)
	}
	w.loadIntoBlocks(data, blockEntities)
	return data, nil
}

// calculateLight calculates the light in the chunk passed and spreads the light of any of the surrounding
// neighbours if they have all chunks loaded around it as a result of the one passed.
func (w *World) calculateLight(c *chunk.Chunk, pos ChunkPos) {
	chunk.FillLight(c)

	for x := int32(-1); x <= 1; x++ {
		for z := int32(-1); z <= 1; z++ {
			// For all of the neighbours of this chunk, if they exist, check if all neighbours of that chunk
			// now exist because of this one.
			centrePos := ChunkPos{pos[0] + x, pos[1] + z}
			neighbour, ok := w.chunks[centrePos]
			if !ok {
				continue
			}
			neighbour.Lock()
			// We first attempt to spread the light of all neighbours into the ones around them.
			w.spreadLight(neighbour.Chunk, centrePos)
			neighbour.Unlock()
		}
	}
	// If the chunk loaded happened to be in the middle of a bunch of other chunks, we are able to spread it
	// right away, so we try to do that.
	w.spreadLight(c, pos)
}

// spreadLight spreads the light from the chunk passed at the position passed to all neighbours if each of
// them is loaded.
func (w *World) spreadLight(c *chunk.Chunk, pos ChunkPos) {
	neighbours, allPresent := make([]*chunk.Chunk, 0, 8), true
	for x := int32(-1); x <= 1; x++ {
		for z := int32(-1); z <= 1; z++ {
			neighbour, ok := w.chunks[ChunkPos{pos[0] + x, pos[1] + z}]
			if !ok {
				allPresent = false
				break
			}
			if x != 0 || z != 0 {
				neighbours = append(neighbours, neighbour.Chunk)
			}
		}
	}
	if allPresent {
		for _, neighbour := range neighbours {
			neighbour.Lock()
		}
		// All neighbours of the current one are present, so we can spread the light from this chunk
		// to all neighbours.
		chunk.SpreadLight(c, neighbours)
		for _, neighbour := range neighbours {
			neighbour.Unlock()
		}
	}
}

// loadIntoBlocks loads the block entity data passed into blocks located in a specific chunk. The blocks that
// have block NBT will then be stored into memory.
func (w *World) loadIntoBlocks(c *chunkData, blockEntityData []map[string]interface{}) {
	c.e = make(map[BlockPos]Block, len(blockEntityData))
	for _, data := range blockEntityData {
		pos := blockPosFromNBT(data)

		id := c.RuntimeID(uint8(pos[0]), uint8(pos[1]), uint8(pos[2]), 0)
		b, ok := blockByRuntimeID(id)
		if !ok {
			w.log.Errorf("error loading block entity data: could not find block state by runtime ID %v", id)
			continue
		}
		if nbt, ok := b.(NBTer); ok {
			b = nbt.DecodeNBT(data).(Block)
		}
		c.e[pos] = b
	}
}

// saveChunk is called when a chunk is removed from the cache. We first compact the chunk, then we write it to
// the provider.
func (w *World) saveChunk(pos ChunkPos, c *chunkData) {
	c.Lock()
	// We allocate a new map for all block entities.
	m := make([]map[string]interface{}, 0, len(c.e))
	for pos, b := range c.e {
		if n, ok := b.(NBTer); ok {
			// Encode the block entities and add the 'x', 'y' and 'z' tags to it.
			data := n.EncodeNBT()
			data["x"], data["y"], data["z"] = int32(pos[0]), int32(pos[1]), int32(pos[2])
			m = append(m, data)
		}
	}
	if !w.rdonly.Load() {
		c.Compact()
		if err := w.provider().SaveChunk(pos, c.Chunk); err != nil {
			w.log.Errorf("error saving chunk %v to provider: %v", pos, err)
		}
		if err := w.provider().SaveEntities(pos, c.entities); err != nil {
			w.log.Errorf("error saving entities in chunk %v to provider: %v", pos, err)
		}
		if err := w.provider().SaveBlockNBT(pos, m); err != nil {
			w.log.Errorf("error saving block NBT in chunk %v to provider: %v", pos, err)
		}
	}
	entities := c.entities
	c.entities = nil
	c.Unlock()

	for _, entity := range entities {
		_ = entity.Close()
	}
}

// initChunkCache initialises the chunk cache of the world to its default values.
func (w *World) initChunkCache() {
	w.chunkMu.Lock()
	w.chunks = make(map[ChunkPos]*chunkData)
	w.chunkMu.Unlock()
}

// CloseChunkCacheJanitor closes the chunk cache janitor of the world. Calling this method will prevent chunks
// from unloading until the World is closed, preventing entities from despawning. As a result, this could lead
// to a memory leak if the size of the world can grow. This method should therefore only be used in places
// where the movement of players is limited to a confined space such as a hub.
func (w *World) CloseChunkCacheJanitor() {
	close(w.stopCacheJanitor)
}

// chunkCacheJanitor runs until the world is closed, cleaning chunks that are no longer in use from the cache.
func (w *World) chunkCacheJanitor() {
	t := time.NewTicker(time.Minute * 5)
	defer t.Stop()

	chunksToRemove := map[ChunkPos]*chunkData{}
	for {
		select {
		case <-t.C:
			w.chunkMu.Lock()
			for pos, c := range w.chunks {
				if len(c.v) == 0 {
					chunksToRemove[pos] = c
					delete(w.chunks, pos)
				}
			}
			w.chunkMu.Unlock()

			for pos, c := range chunksToRemove {
				w.saveChunk(pos, c)
				delete(chunksToRemove, pos)
			}
		case <-w.stopTick.Done():
			return
		case <-w.stopCacheJanitor:
			return
		}
	}
}

// chunkData represents the data of a chunk including the block entities and viewers. This data is protected
// by the mutex present in the chunk.Chunk held.
type chunkData struct {
	*chunk.Chunk
	e        map[BlockPos]Block
	v        []Viewer
	entities []Entity
}

// newChunkData returns a new chunkData wrapper around the chunk.Chunk passed.
func newChunkData(c *chunk.Chunk) *chunkData {
	return &chunkData{Chunk: c, e: map[BlockPos]Block{}}
}
