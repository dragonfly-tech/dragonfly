package block

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/block/model"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

// Ladder is a wooden block used for climbing walls either vertically or horizontally. They can be placed only on
// the sides of other blocks.
type Ladder struct {
	transparent

	// Facing is the side of the block the ladder is currently attached to.
	Facing cube.Direction
}

// NeighbourUpdateTick ...
func (l Ladder) NeighbourUpdateTick(pos, _ cube.Pos, w *world.World) {
	if _, ok := w.Block(pos.Side(l.Facing.Opposite().Face())).(LightDiffuser); ok {
		w.BreakBlock(pos)
	}
}

// UseOnBlock ...
func (l Ladder) UseOnBlock(pos cube.Pos, face cube.Face, _ mgl64.Vec3, w *world.World, user item.User, ctx *item.UseContext) bool {
	pos, face, used := firstReplaceable(w, pos, face, l)
	if !used {
		return false
	}
	if face == cube.FaceUp || face == cube.FaceDown {
		return false
	}
	if _, ok := w.Block(pos.Side(face.Opposite())).(LightDiffuser); ok {
		found := false
		for _, i := range []cube.Face{cube.FaceSouth, cube.FaceNorth, cube.FaceEast, cube.FaceWest} {
			if _, ok := w.Block(pos.Side(i)).(LightDiffuser); !ok {
				found = true
				face = i.Opposite()
				break
			}
		}
		if !found {
			return false
		}
	}
	l.Facing = face.Direction()

	place(w, pos, l, user, ctx)
	return placed(ctx)
}

// EntityCollide ...
func (Ladder) EntityCollide(e world.Entity) {
	if fallEntity, ok := e.(FallDistanceEntity); ok {
		fallEntity.ResetFallDistance()
	}
}

// CanDisplace ...
func (Ladder) CanDisplace(b world.Liquid) bool {
	_, water := b.(Water)
	return water
}

// SideClosed ...
func (Ladder) SideClosed(cube.Pos, cube.Pos, *world.World) bool {
	return false
}

// BreakInfo ...
func (l Ladder) BreakInfo() BreakInfo {
	return newBreakInfo(0.4, alwaysHarvestable, axeEffective, oneOf(l))
}

// EncodeItem ...
func (Ladder) EncodeItem() (name string, meta int16) {
	return "minecraft:ladder", 0
}

// EncodeBlock ...
func (l Ladder) EncodeBlock() (string, map[string]interface{}) {
	return "minecraft:ladder", map[string]interface{}{"facing_direction": int32(l.Facing + 2)}
}

// Model ...
func (l Ladder) Model() world.BlockModel {
	return model.Ladder{Facing: l.Facing}
}

// allLadders ...
func allLadders() (b []world.Block) {
	for i := cube.Direction(0); i <= 3; i++ {
		b = append(b, Ladder{Facing: i})
	}
	return
}
