package session

import (
	"github.com/df-mc/dragonfly/dragonfly/world"
)

// entityMetadata represents a map that holds metadata associated with an entity. The data held in the map
// depends on the entity and varies on a per-entity basis.
type entityMetadata map[uint32]interface{}

// defaultEntityMetadata returns an entity metadata object with default values. It is equivalent to setting
// all properties to their default values and disabling all flags.
func defaultEntityMetadata(e world.Entity) entityMetadata {
	m := entityMetadata{}
	m.setFlag(dataKeyFlags, dataFlagAffectedByGravity)

	bb := e.AABB()
	m[dataKeyBoundingBoxWidth] = float32(bb.Width())
	m[dataKeyBoundingBoxHeight] = float32(bb.Height())
	m[dataKeyPotionColour] = int32(0)
	m[dataKeyPotionAmbient] = byte(0)
	m[dataKeyColour] = byte(0)

	return m
}

// setFlag sets a flag with a specific index in the int64 stored in the entity metadata map to the value
// passed. It is typically used for entity metadata flags.
func (m entityMetadata) setFlag(key uint32, index uint8) {
	if v, ok := m[key]; !ok {
		m[key] = int64(0) ^ (1 << uint64(index))
	} else {
		m[key] = v.(int64) ^ (1 << uint64(index))
	}
}

//noinspection GoUnusedConst
const (
	dataKeyFlags = iota
	dataKeyHealth
	dataKeyVariant
	dataKeyColour
	dataKeyNameTag
	dataKeyOwnerRuntimeID
	dataKeyTargetRuntimeID
	dataKeyAir
	dataKeyPotionColour
	dataKeyPotionAmbient
	dataKeyScale             = 38
	dataKeyBoundingBoxWidth  = 53
	dataKeyBoundingBoxHeight = 54
)

//noinspection GoUnusedConst
const (
	dataFlagOnFire = iota
	dataFlagSneaking
	dataFlagRiding
	dataFlagSprinting
	dataFlagUsingItem
	dataFlagInvisible
	dataFlagNoAI              = 16
	dataFlagBreathing         = 35
	dataFlagAffectedByGravity = 48
	dataFlagSwimming          = 56
)
