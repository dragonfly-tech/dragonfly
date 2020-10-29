package block

import (
	"github.com/df-mc/dragonfly/dragonfly/block/colour"
	"github.com/df-mc/dragonfly/dragonfly/item"
	"github.com/df-mc/dragonfly/dragonfly/world"
	"github.com/go-gl/mathgl/mgl64"
)

// GlazedTerracotta is a vibrant solid block that comes in the 16 regular dye colours.
type GlazedTerracotta struct {
	noNBT
	solid
	bassDrum

	// Colour specifies the colour of the block.
	Colour colour.Colour
	// Facing specifies the face of the block.
	Facing world.Direction
}

// BreakInfo ...
func (t GlazedTerracotta) BreakInfo() BreakInfo {
	return BreakInfo{
		Hardness:    1.4,
		Harvestable: pickaxeHarvestable,
		Effective:   pickaxeEffective,
		Drops:       simpleDrops(item.NewStack(t, 1)),
	}
}

// EncodeItem ...
func (t GlazedTerracotta) EncodeItem() (id int32, meta int16) {
	// Item ID for glazed terracotta is equal to 220 + colour number.
	if t.Colour == colour.Purple() {
		return 219, 0
	}
	return int32(220 + t.Colour.Uint8()), meta
}

// EncodeBlock ...
func (t GlazedTerracotta) EncodeBlock() (name string, properties map[string]interface{}) {
	colourName := t.Colour.String()
	if t.Colour == colour.LightGrey() {
		// Light grey is actually called "silver" in the block state. Mojang pls.
		colourName = "silver"
	}
	return "minecraft:" + colourName + "_glazed_terracotta", map[string]interface{}{"facing_direction": int32(2 + t.Facing)}
}

// Hash ...
func (t GlazedTerracotta) Hash() uint64 {
	return hashGlazedTerracotta | (uint64(t.Facing) << 32) | (uint64(t.Colour.Uint8()) << 34)
}

// UseOnBlock ensures the proper facing is used when placing a glazed terracotta block, by using the opposite of the player.
func (t GlazedTerracotta) UseOnBlock(pos world.BlockPos, face world.Face, _ mgl64.Vec3, w *world.World, user item.User, ctx *item.UseContext) (used bool) {
	pos, _, used = firstReplaceable(w, pos, face, t)
	if !used {
		return
	}
	t.Facing = user.Facing().Opposite()

	place(w, pos, t, user, ctx)
	return placed(ctx)
}

// allGlazedTerracotta returns glazed terracotta blocks with all possible colours.
func allGlazedTerracotta() (b []world.Block) {
	for dir := world.Direction(0); dir < 4; dir++ {
		for _, c := range colour.All() {
			b = append(b, GlazedTerracotta{Colour: c, Facing: dir})
		}
	}
	return b
}
