package block

import (
	"github.com/df-mc/dragonfly/dragonfly/block/model"
	"github.com/df-mc/dragonfly/dragonfly/entity/physics"
	"github.com/df-mc/dragonfly/dragonfly/item"
	"github.com/df-mc/dragonfly/dragonfly/item/tool"
	"github.com/df-mc/dragonfly/dragonfly/world"
	"github.com/go-gl/mathgl/mgl64"
)

// RedSandstoneSlab is a half block that allows entities to walk up blocks without jumping.
type RedSandstoneSlab struct {
	noNBT

	// Top specifies if the slab is in the top part of the block.
	Top bool
	// Double specifies if the slab is a double slab. These double slabs can be made by placing another slab
	// on an existing slab.
	Double bool
}

// Model ...
func (s RedSandstoneSlab) Model() world.BlockModel {
	return model.Slab{Double: s.Double, Top: s.Top}
}

// UseOnBlock handles the placement of slabs with relation to them being upside down or not and handles slabs
// being turned into double slabs.
func (s RedSandstoneSlab) UseOnBlock(pos world.BlockPos, face world.Face, clickPos mgl64.Vec3, w *world.World, user item.User, ctx *item.UseContext) (used bool) {
	clickedBlock := w.Block(pos)
	if clickedSlab, ok := clickedBlock.(RedSandstoneSlab); ok && !s.Double {
		if (face == world.FaceUp && !clickedSlab.Double && !clickedSlab.Top) ||
			(face == world.FaceDown && !clickedSlab.Double && clickedSlab.Top) {
			// A half slab of the same type was clicked at the top, so we can make it full.
			clickedSlab.Double = true

			place(w, pos, clickedSlab, user, ctx)
			return placed(ctx)
		}
	}
	if sideSlab, ok := w.Block(pos.Side(face)).(RedSandstoneSlab); ok && !replaceableWith(w, pos, s) && !s.Double {
		// The block on the side of the one clicked was a slab and the block clicked was not replaceableWith, so
		// the slab on the side must've been half and may now be filled if the wood types are the same.
		if !sideSlab.Double {
			sideSlab.Double = true

			place(w, pos.Side(face), sideSlab, user, ctx)
			return placed(ctx)
		}
	}
	pos, face, used = firstReplaceable(w, pos, face, s)
	if !used {
		return
	}
	if face == world.FaceDown || (clickPos[1] > 0.5 && face != world.FaceUp) {
		s.Top = true
	}

	place(w, pos, s, user, ctx)
	return placed(ctx)
}

// BreakInfo ...
func (s RedSandstoneSlab) BreakInfo() BreakInfo {
	return BreakInfo{
		Hardness:    2,
		Harvestable: pickaxeHarvestable,
		Effective:   pickaxeEffective,
		Drops: func(t tool.Tool) []item.Stack {
			if s.Double {
				s.Double = false
				// If the slab is double, it should drop two single slabs.
				return []item.Stack{item.NewStack(s, 2)}
			}
			return []item.Stack{item.NewStack(s, 1)}
		},
	}
}

// LightDiffusionLevel returns 0 if the slab is a half slab, or 15 if it is double.
func (s RedSandstoneSlab) LightDiffusionLevel() uint8 {
	if s.Double {
		return 15
	}
	return 0
}

// AABB ...
func (s RedSandstoneSlab) AABB(world.BlockPos, *world.World) []physics.AABB {
	if s.Double {
		return []physics.AABB{physics.NewAABB(mgl64.Vec3{}, mgl64.Vec3{1, 1, 1})}
	}
	if s.Top {
		return []physics.AABB{physics.NewAABB(mgl64.Vec3{0, 0.5, 0}, mgl64.Vec3{1, 1, 1})}
	}
	return []physics.AABB{physics.NewAABB(mgl64.Vec3{}, mgl64.Vec3{1, 0.5, 1})}
}

// EncodeItem ...
func (s RedSandstoneSlab) EncodeItem() (id int32, meta int16) {
	if s.Double {
		return 181, 0
	}
	return 182, 0
}

// EncodeBlock ...
func (s RedSandstoneSlab) EncodeBlock() (name string, properties map[string]interface{}) {
	if s.Double {
		return "minecraft:double_stone_slab2", map[string]interface{}{"top_slot_bit": s.Top, "stone_slab_type_2": "red_sandstone"}
	}
	return "minecraft:stone_slab2", map[string]interface{}{"top_slot_bit": s.Top, "stone_slab_type_2": "red_sandstone"}
}

// Hash ...
func (s RedSandstoneSlab) Hash() uint64 {
	return hashRedSandstoneSlab | (uint64(boolByte(s.Top)) << 32) | (uint64(boolByte(s.Double)) << 33)
}

// CanDisplace ...
func (s RedSandstoneSlab) CanDisplace(b world.Liquid) bool {
	_, ok := b.(Water)
	return !s.Double && ok
}

// SideClosed ...
func (s RedSandstoneSlab) SideClosed(pos, side world.BlockPos, _ *world.World) bool {
	// Only returns true if the side is below the slab and if the slab is not upside down.
	return !s.Top && side[1] == pos[1]-1
}

// allRedSandstoneSlabs returns all states of red sandstone slabs.
func allRedSandstoneSlabs() (slabs []world.Block) {
	f := func(double bool, upsideDown bool) {
		slabs = append(slabs, RedSandstoneSlab{Double: double, Top: upsideDown})
	}
	f(false, false)
	f(false, true)
	f(true, false)
	f(true, true)
	return
}
