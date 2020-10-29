package session

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/df-mc/dragonfly/dragonfly/item/inventory"
	"github.com/df-mc/dragonfly/dragonfly/player/chat"
	"github.com/df-mc/dragonfly/dragonfly/player/form"
	"github.com/df-mc/dragonfly/dragonfly/world"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/text"
	"github.com/sirupsen/logrus"
	"go.uber.org/atomic"
	"sync"
	"time"
)

// Session handles incoming packets from connections and sends outgoing packets by providing a thin layer
// of abstraction over direct packets. A Session basically 'controls' an entity.
type Session struct {
	log *logrus.Logger

	c        Controllable
	conn     *minecraft.Conn
	handlers map[uint32]packetHandler

	// onStop is called when the session is stopped. The controllable passed is the controllable that the
	// session controls.
	onStop func(controllable Controllable)

	scoreboardObj atomic.String

	chunkBuf                    *bytes.Buffer
	chunkLoader                 *world.Loader
	chunkRadius, maxChunkRadius int32

	teleportMu  sync.Mutex
	teleportPos *mgl64.Vec3

	// currentEntityRuntimeID holds the runtime ID assigned to the last entity. It is incremented for every
	// entity spawned to the session.
	currentEntityRuntimeID atomic.Uint64
	entityMutex            sync.RWMutex
	// entityRuntimeIDs holds a list of all runtime IDs of entities spawned to the session.
	entityRuntimeIDs map[world.Entity]uint64
	entities         map[uint64]world.Entity

	// heldSlot is the slot in the inventory that the controllable is holding.
	heldSlot         *atomic.Uint32
	inv, offHand, ui *inventory.Inventory
	armour           *inventory.Armour

	openedWindowID                 atomic.Uint32
	inTransaction, containerOpened atomic.Bool
	openedWindow, openedPos        atomic.Value
	swingingArm                    atomic.Bool

	// adventureFlags are the adventure flags for the session.
	adventureFlags *atomic.Uint32

	blobMu                sync.Mutex
	blobs                 map[uint64][]byte
	openChunkTransactions []map[uint64]struct{}
	invOpened             bool
}

// Nop represents a no-operation session. It does not do anything when sending a packet to it.
var Nop = &Session{}

// session is a slice of all open sessions. It is protected by the sessionMu, which must be locked whenever
// accessing the value.
var sessions []*Session
var sessionMu sync.Mutex

// selfEntityRuntimeID is the entity runtime (or unique) ID of the controllable that the session holds.
const selfEntityRuntimeID = 1

// ErrSelfRuntimeID is an error returned during packet handling for fields that refer to the player itself and
// must therefore always be 1.
var ErrSelfRuntimeID = errors.New("invalid entity runtime ID: runtime ID for self must always be 1")

// New returns a new session using a controllable entity. The session will control this entity using the
// packets that it receives.
// New takes the connection from which to accept packets. It will start handling these packets after a call to
// Session.Start().
func New(conn *minecraft.Conn, maxChunkRadius int, log *logrus.Logger) *Session {
	r := conn.ChunkRadius()
	if r > maxChunkRadius {
		r = maxChunkRadius
		_ = conn.WritePacket(&packet.ChunkRadiusUpdated{ChunkRadius: int32(r)})
	}

	s := &Session{
		chunkBuf:               bytes.NewBuffer(make([]byte, 0, 4096)),
		openChunkTransactions:  make([]map[uint64]struct{}, 0, 8),
		ui:                     inventory.New(51, nil),
		handlers:               map[uint32]packetHandler{},
		entityRuntimeIDs:       map[world.Entity]uint64{},
		entities:               map[uint64]world.Entity{},
		blobs:                  map[uint64][]byte{},
		chunkRadius:            int32(r),
		maxChunkRadius:         int32(maxChunkRadius),
		conn:                   conn,
		log:                    log,
		currentEntityRuntimeID: *atomic.NewUint64(1),
		heldSlot:               atomic.NewUint32(0),
		adventureFlags:         atomic.NewUint32(0),
	}
	s.openedWindow.Store(inventory.New(1, nil))
	s.openedPos.Store(world.BlockPos{})

	s.registerHandlers()
	return s
}

// Start makes the session start handling incoming packets from the client and initialises the controllable of
// the session in the world.
// The function passed will be called when the session stops running.
func (s *Session) Start(c Controllable, w *world.World, onStop func(controllable Controllable)) {
	s.onStop = onStop
	s.c = c
	s.entityRuntimeIDs[c] = selfEntityRuntimeID
	s.entities[selfEntityRuntimeID] = c

	s.chunkLoader = world.NewLoader(int(s.chunkRadius), w, s)
	s.chunkLoader.Move(w.Spawn().Vec3Middle())

	s.initPlayerList()

	w.AddEntity(s.c)
	s.c.SetGameMode(w.DefaultGameMode())
	s.SendAvailableCommands()
	s.SendSpeed(0.1)

	go s.handlePackets()

	yellow := text.Yellow()
	chat.Global.Println(yellow(s.conn.IdentityData().DisplayName, "has joined the game"))

	s.writePacket(&packet.CreativeContent{Items: creativeItems()})
}

// Close closes the session, which in turn closes the controllable and the connection that the session
// manages.
func (s *Session) Close() error {
	s.closeCurrentContainer()

	_ = s.conn.Close()
	_ = s.chunkLoader.Close()
	_ = s.c.Close()

	yellow := text.Yellow()
	chat.Global.Println(yellow(s.conn.IdentityData().DisplayName, "has left the game"))

	if s.c.World() != nil {
		s.c.World().RemoveEntity(s.c)
	}

	// This should always be called last due to the timing of the removal of entity runtime IDs.
	s.closePlayerList()

	s.entityMutex.Lock()
	s.entityRuntimeIDs = map[world.Entity]uint64{}
	s.entities = map[uint64]world.Entity{}
	s.entityMutex.Unlock()

	if s.onStop != nil {
		s.onStop(s.c)
		s.onStop = nil
	}
	return nil
}

// CloseConnection closes the underlying connection of the session so that the session ends up being closed
// eventually.
func (s *Session) CloseConnection() {
	_ = s.conn.Close()
}

// Latency returns the latency of the connection.
func (s *Session) Latency() time.Duration {
	return s.conn.Latency()
}

// handlePackets continuously handles incoming packets from the connection. It processes them accordingly.
// Once the connection is closed, handlePackets will return.
func (s *Session) handlePackets() {
	c := make(chan struct{})
	defer func() {
		// If this function ends up panicking, we don't want to call s.Close() as it may cause the entire
		// server to freeze without printing the actual panic message.
		// Instead, we check if there is a panic to recover, and just propagate the panic if this does happen
		// to be the case.
		if err := recover(); err != nil {
			panic(err)
		}
		c <- struct{}{}
		_ = s.Close()
	}()
	go s.sendChunks(c)
	for {
		pk, err := s.conn.ReadPacket()
		if err != nil {
			return
		}
		if err := s.handlePacket(pk); err != nil {
			// An error occurred during the handling of a packet. Print the error and stop handling any more
			// packets.
			s.log.Debugf("failed processing packet from %v (%v): %v\n", s.conn.RemoteAddr(), s.c.Name(), err)
			return
		}
	}
}

// sendChunks continuously sends chunks to the player, until a value is sent to the closeChan passed.
func (s *Session) sendChunks(stop <-chan struct{}) {
	const maxChunkTransactions = 8
	t := time.NewTicker(time.Second / 20)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			if s.chunkLoader.World() != s.c.World() {
				s.chunkLoader.ChangeWorld(s.c.World())
			}
			s.blobMu.Lock()
			toLoad := maxChunkTransactions - len(s.openChunkTransactions)
			s.blobMu.Unlock()

			if toLoad > 4 {
				toLoad = 4
			}
			if err := s.chunkLoader.Load(toLoad); err != nil {
				// The world was closed. This should generally never happen.
				s.log.Errorf("error loading chunk: %v", err)
				return
			}
		case <-stop:
			return
		}
	}
}

// handlePacket handles an incoming packet, processing it accordingly. If the packet had invalid data or was
// otherwise not valid in its context, an error is returned.
func (s *Session) handlePacket(pk packet.Packet) error {
	handler, ok := s.handlers[pk.ID()]
	if !ok {
		s.log.Debugf("unhandled packet %T%v from %v\n", pk, fmt.Sprintf("%+v", pk)[1:], s.conn.RemoteAddr())
		return nil
	}
	if handler == nil {
		// A nil handler means it was explicitly unhandled.
		return nil
	}
	if err := handler.Handle(pk, s); err != nil {
		return fmt.Errorf("%T: %w", pk, err)
	}
	return nil
}

// registerHandlers registers all packet handlers found in the packetHandler package.
func (s *Session) registerHandlers() {
	s.handlers = map[uint32]packetHandler{
		packet.IDActorEvent:            nil,
		packet.IDActorFall:             nil,
		packet.IDAnimate:               nil,
		packet.IDBlockPickRequest:      &BlockPickRequestHandler{},
		packet.IDBossEvent:             nil,
		packet.IDClientCacheBlobStatus: &ClientCacheBlobStatusHandler{},
		packet.IDCommandRequest:        &CommandRequestHandler{},
		packet.IDContainerClose:        &ContainerCloseHandler{},
		packet.IDEmote:                 &EmoteHandler{},
		packet.IDEmoteList:             nil,
		packet.IDInteract:              &InteractHandler{},
		packet.IDInventoryTransaction:  &InventoryTransactionHandler{},
		packet.IDItemStackRequest:      &ItemStackRequestHandler{changes: make(map[byte]map[byte]protocol.StackResponseSlotInfo), responseChanges: map[int32]map[byte]map[byte]responseChange{}},
		packet.IDLevelSoundEvent:       nil,
		packet.IDMobEquipment:          &MobEquipmentHandler{},
		packet.IDModalFormResponse:     &ModalFormResponseHandler{forms: make(map[uint32]form.Form)},
		packet.IDMovePlayer:            nil,
		packet.IDPlayerAction:          &PlayerActionHandler{},
		packet.IDPlayerAuthInput:       &PlayerAuthInputHandler{},
		packet.IDRequestChunkRadius:    &RequestChunkRadiusHandler{},
		packet.IDRespawn:               &RespawnHandler{},
		packet.IDText:                  &TextHandler{},
		packet.IDTickSync:              nil,
		packet.IDAdventureSettings:     &AdventureSettingsHandler{},
		packet.IDPlayerSkin:            &PlayerSkinHandler{},
	}
}

// writePacket writes a packet to the session's connection if it is not Nop.
func (s *Session) writePacket(pk packet.Packet) {
	if s == Nop {
		return
	}
	_ = s.conn.WritePacket(pk)
}

// initPlayerList initialises the player list of the session and sends the session itself to all other
// sessions currently open.
func (s *Session) initPlayerList() {
	sessionMu.Lock()
	sessions = append(sessions, s)
	for _, session := range sessions {
		// AddStack the player of the session to all sessions currently open, and add the players of all sessions
		// currently open to the player list of the new session.
		session.addToPlayerList(s)
		s.addToPlayerList(session)
	}
	sessionMu.Unlock()
}

// closePlayerList closes the player list of the session and removes the session from the player list of all
// other sessions.
func (s *Session) closePlayerList() {
	sessionMu.Lock()
	n := make([]*Session, 0, len(sessions)-1)
	for _, session := range sessions {
		if session != s {
			n = append(n, session)
		}
		// Remove the player of the session from the player list of all other sessions.
		session.removeFromPlayerList(s)
	}
	sessions = n
	sessionMu.Unlock()
}

// connection returns the minecraft connection.
//lint:ignore U1000 Function is used using compiler directives.
//noinspection GoUnusedFunction
func (s *Session) connection() *minecraft.Conn {
	return s.conn
}
