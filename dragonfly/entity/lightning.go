package entity

import (
	"github.com/df-mc/dragonfly/dragonfly/entity/physics"
	"github.com/df-mc/dragonfly/dragonfly/entity/state"
	"github.com/df-mc/dragonfly/dragonfly/world"
	"github.com/go-gl/mathgl/mgl64"
	"sync/atomic"
)

// Lightning is a lethal element to thunderstorms. Lightning momentarily increases the sky light's brightness to slightly greater than full daylight.
type Lightning struct {
	pos atomic.Value
}

// NewLightning creates a lightning entity. The lightning entity will be positioned at the position passed.
func NewLightning(pos mgl64.Vec3) *Lightning {
	li := &Lightning{}
	li.pos.Store(pos)

	return li
}

// Position returns the current position of the lightning entity.
func (li *Lightning) Position() mgl64.Vec3 {
	return li.pos.Load().(mgl64.Vec3)
}

// World returns the world that the lightning entity is currently in, or nil if it is not added to a world.
func (li *Lightning) World() *world.World {
	w, _ := world.OfEntity(li)
	return w
}

// Velocity ...
func (Lightning) Velocity() mgl64.Vec3 {
	return mgl64.Vec3{}
}

// SetVelocity ...
func (Lightning) SetVelocity(mgl64.Vec3) {}

// Yaw always returns 0.
func (Lightning) Yaw() float64 {
	return 0
}

// Pitch always returns 0.
func (Lightning) Pitch() float64 {
	return 0
}

// AABB ...
func (Lightning) AABB() physics.AABB {
	return physics.NewAABB(mgl64.Vec3{}, mgl64.Vec3{})
}

// State ...
func (Lightning) State() []state.State {
	return nil
}

// Close closes the lighting.
func (li *Lightning) Close() error {
	if li.World() != nil {
		li.World().RemoveEntity(li)
	}

	return nil
}

// OnGround ...
func (Lightning) OnGround() bool {
	return false
}

// EncodeEntity ...
func (Lightning) EncodeEntity() string {
	return "minecraft:lightning_bolt"
}
