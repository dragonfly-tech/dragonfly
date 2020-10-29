package session

import (
	"github.com/df-mc/dragonfly/dragonfly/item"
	"github.com/df-mc/dragonfly/dragonfly/player/form"
	"github.com/df-mc/dragonfly/dragonfly/player/skin"
	"github.com/df-mc/dragonfly/dragonfly/world"
	"github.com/df-mc/dragonfly/dragonfly/world/gamemode"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/google/uuid"
)

// Controllable represents an entity that may be controlled by a Session. Generally, a Controllable is
// implemented in the form of a Player.
// Methods in Controllable will be added as Session needs them in order to handle packets.
type Controllable interface {
	world.Entity
	item.Carrier
	form.Submitter
	SetHeldItems(right, left item.Stack)

	Move(deltaPos mgl64.Vec3)
	Speed() float64
	EyeHeight() float64
	Rotate(deltaYaw, deltaPitch float64)

	Chat(msg ...interface{})
	ExecuteCommand(commandLine string)
	GameMode() gamemode.GameMode
	SetGameMode(mode gamemode.GameMode)

	UseItem()
	ReleaseItem()
	UseItemOnBlock(pos world.BlockPos, face world.Face, clickPos mgl64.Vec3)
	UseItemOnEntity(e world.Entity)
	BreakBlock(pos world.BlockPos)
	PickBlock(pos world.BlockPos)
	AttackEntity(e world.Entity)
	Drop(s item.Stack) (n int)

	Respawn()

	StartSneaking()
	Sneaking() bool
	StopSneaking()
	StartSprinting()
	Sprinting() bool
	StopSprinting()
	StartSwimming()
	Swimming() bool
	StopSwimming()
	Flying() bool
	CanFly() bool
	SetCanFly(bool)
	SetFlying(bool)

	StartBreaking(pos world.BlockPos, face world.Face)
	ContinueBreaking(face world.Face)
	FinishBreaking()
	AbortBreaking()

	Exhaust(points float64)

	// Name returns the display name of the controllable. This name is shown in-game to other viewers of the
	// world.
	Name() string
	// UUID returns the UUID of the controllable. It must be unique for all controllable entities present in
	// the server.
	UUID() uuid.UUID
	// XUID returns the XBOX Live User ID of the controllable. Every controllable must have one of these if
	// they are authenticated via XBOX Live, as they must be connected to an XBOX Live account.
	XUID() string
	// Skin returns the skin of the controllable. Each controllable must have a skin, as it defines how the
	// entity looks in the world.
	Skin() skin.Skin
	SetSkin(skin.Skin)
}
