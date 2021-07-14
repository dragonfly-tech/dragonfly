package state

import "image/color"

// State represents a part of the state of an entity. Entities may hold a combination of these to indicate
// things such as whether it is sprinting or on fire.
type State interface {
	__()
}

// Sneaking makes the entity show up as if it is sneaking.
type Sneaking struct{}

// Sprinting makes an entity show up as if it is sprinting: Particles will show up when the entity moves
// around the world.
type Sprinting struct{}

// Swimming makes an entity show up as if it is swimming.
type Swimming struct{}

// Breathing makes an entity breath: This state will not show up for entities other than players.
type Breathing struct{}

// Invisible makes an entity invisible, so that other players won't be able to see it.
type Invisible struct{}

// Immobile makes the entity able to look around but they are not able to move from their position.
type Immobile struct{}

// EffectBearing makes an entity show up as if it is bearing effects. Coloured particles will be shown around
// the player.
type EffectBearing struct {
	// ParticleColour holds the colour of the particles that are displayed around the entity.
	ParticleColour color.RGBA
	// Ambient specifies if the effects are ambient. If true, the particles will be shown less frequently
	// around the entity.
	Ambient bool
}

// Named makes an entity show a specific name tag above it.
type Named struct {
	// NameTag is the name displayed. This name may have colour codes, newlines etc in it, much like a normal
	// message.
	NameTag string
}

// UsingItem makes an entity show itself as using the item held in its hand.
type UsingItem struct{}

// OnFire makes an entity show itself as on fire.
type OnFire struct{}

// Scaled makes an entity show up with a different scale.
type Scaled struct {
	// Scale the size multiplier of the entity. 1 is the default, 0 is a completely invisible entity.
	Scale float64
}

// CanClimb allows an entity to climb ladders & vines.
type CanClimb struct{}

func (Sneaking) __()      {}
func (Swimming) __()      {}
func (Breathing) __()     {}
func (Sprinting) __()     {}
func (Invisible) __()     {}
func (Immobile) __()      {}
func (Named) __()         {}
func (EffectBearing) __() {}
func (UsingItem) __()     {}
func (OnFire) __()        {}
func (Scaled) __()        {}
func (CanClimb) __()      {}
