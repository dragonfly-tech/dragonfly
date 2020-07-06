package block

import (
	"github.com/df-mc/dragonfly/dragonfly/block/colour"
	"github.com/df-mc/dragonfly/dragonfly/entity/physics"
	"github.com/df-mc/dragonfly/dragonfly/item/tool"
	"github.com/df-mc/dragonfly/dragonfly/world"
)

// StainedGlassPane is a transparent block that can be used as a more efficient alternative to glass blocks.
type StainedGlassPane struct {
	Colour colour.Colour
}

// BreakInfo ...
func (p StainedGlassPane) BreakInfo() BreakInfo {
	return BreakInfo{
		Hardness: 0.3,
		Harvestable: func(t tool.Tool) bool {
			return true // TODO(lhochbaum): Glass panes can be silk touched, implement silk touch.
		},
		Effective: nothingEffective,
		Drops:     simpleDrops(),
	}
}

// EncodeItem ...
func (p StainedGlassPane) EncodeItem() (id int32, meta int16) {
	return 160, int16(p.Colour.Uint8())
}

// EncodeBlock ...
func (p StainedGlassPane) EncodeBlock() (name string, properties map[string]interface{}) {
	colourName := p.Colour.String()
	if p.Colour == colour.LightGrey() {
		// And here we go again. Light grey is actually called "silver".
		colourName = "silver"
	}
	return "minecraft:stained_glass_pane", map[string]interface{}{"color": colourName}
}

// allStainedGlassPane returns stained glass panes with all possible colours.
func allStainedGlassPane() []world.Block {
	b := make([]world.Block, 0, 16)
	for _, c := range colour.All() {
		b = append(b, StainedGlassPane{Colour: c})
	}
	return b
}

// AABB adjusts bounding box of the glass pane.
func (p StainedGlassPane) AABB(pos world.BlockPos, w *world.World) []physics.AABB {
	return calculateThinBounds(pos, w)
}