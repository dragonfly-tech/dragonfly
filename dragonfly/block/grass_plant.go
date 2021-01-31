package block

import (
	"github.com/df-mc/dragonfly/dragonfly/block/grass"
	"github.com/df-mc/dragonfly/dragonfly/item"
	"github.com/df-mc/dragonfly/dragonfly/item/tool"
	"github.com/df-mc/dragonfly/dragonfly/world"
	"github.com/go-gl/mathgl/mgl64"
	"math/rand"
)

type GrassPlant struct {
	noNBT
	transparent
	empty

	UpperBit bool

	grass.Grass
}

// BreakInfo ...
func (g GrassPlant) BreakInfo() BreakInfo {
	return BreakInfo{
		Hardness:    0,
		Harvestable: alwaysHarvestable,
		Effective:   nothingEffective,
		Drops: func(t tool.Tool) []item.Stack {
			if g.Grass == grass.NetherSprouts() {
				return []item.Stack{item.NewStack(g, 1)}
			}
			if rand.Float32() > 0.57 {
				return []item.Stack{item.NewStack(WheatSeeds{}, 1)}
			}
			return []item.Stack{}
		},
	}
}

// NeighbourUpdateTick ...
func (g GrassPlant) NeighbourUpdateTick(pos, _ world.BlockPos, w *world.World) {
	if p, ok := w.Block(pos).(GrassPlant); ok {
		if p.Grass == grass.TallGrass() || p.Grass == grass.LargeFern() {
			if _, ok := w.Block(pos.Side(world.FaceDown)).(GrassPlant); !ok && p.UpperBit {
				w.BreakBlock(pos)
			} else if _, ok := w.Block(pos.Side(world.FaceUp)).(GrassPlant); !ok {
				w.BreakBlock(pos)
			}
		}
		return
	}

	if _, ok := w.Block(pos.Side(world.FaceDown)).(Grass); !ok {
		w.BreakBlock(pos)
	}
}

// HasLiquidDrops ...
func (g GrassPlant) HasLiquidDrops() bool {
	return true
}

// UseOnBlock ...
func (g GrassPlant) UseOnBlock(pos world.BlockPos, face world.Face, _ mgl64.Vec3, w *world.World, user item.User, ctx *item.UseContext) bool {
	pos, _, used := firstReplaceable(w, pos, face, g)
	if !used {
		return false
	}
	if _, ok := w.Block(pos.Side(world.FaceDown)).(Grass); !ok {
		return false
	}

	place(w, pos, g, user, ctx)
	if g.Grass == grass.TallGrass() || g.Grass == grass.LargeFern() {
		place(w, pos.Side(world.FaceUp), GrassPlant{Grass: g.Grass, UpperBit: true}, user, ctx)
	}
	return placed(ctx)
}

// EncodeItem ...
func (g GrassPlant) EncodeItem() (id int32, meta int16) {
	switch g.Grass {
	case grass.SmallGrass():
		return 31, 1
	case grass.Fern():
		return 31, 2
	case grass.TallGrass():
		return 175, 2
	case grass.LargeFern():
		return 175, 3
	case grass.NetherSprouts():
		return 760, 0
	}
	panic("should never happen")
}

// EncodeBlock ...
func (g GrassPlant) EncodeBlock() (name string, properties map[string]interface{}) {
	switch g.Grass {
	case grass.SmallGrass():
		return "minecraft:tallgrass", map[string]interface{}{"tall_grass_type": "default"}
	case grass.Fern():
		return "minecraft:tallgrass", map[string]interface{}{"tall_grass_type": "fern"}
	case grass.TallGrass():
		return "minecraft:double_plant", map[string]interface{}{"double_plant_type": "grass", "upper_block_bit": g.UpperBit}
	case grass.LargeFern():
		return "minecraft:double_plant", map[string]interface{}{"double_plant_type": "fern", "upper_block_bit": g.UpperBit}
	case grass.NetherSprouts():
		return "minecraft:nether_sprouts", map[string]interface{}{}
	}
	panic("should never happen")
}

// allGrassPlants ...
func allGrassPlants() (grasses []canEncode) {
	for _, g := range grass.All() {
		grasses = append(grasses, GrassPlant{Grass: g, UpperBit: false})
		grasses = append(grasses, GrassPlant{Grass: g, UpperBit: true})
	}
	return
}
