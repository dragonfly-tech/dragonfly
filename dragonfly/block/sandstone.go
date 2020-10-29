package block

import (
	"github.com/df-mc/dragonfly/dragonfly/item"
	"github.com/df-mc/dragonfly/dragonfly/world"
)

// Sandstone is a solid block commonly found in deserts and beaches underneath sand.
type Sandstone struct {
	noNBT
	solid
	Red  bool
	Data int16
}

// BreakInfo ...
func (s Sandstone) BreakInfo() BreakInfo {
	if s.Data == 3 {
		return BreakInfo{
			Hardness:    2,
			Harvestable: pickaxeHarvestable,
			Effective:   pickaxeEffective,
			Drops:       simpleDrops(item.NewStack(s, 1)),
		}
	} else {
		return BreakInfo{
			Hardness:    0.8,
			Harvestable: pickaxeHarvestable,
			Effective:   pickaxeEffective,
			Drops:       simpleDrops(item.NewStack(s, 1)),
		}
	}
}

// EncodeItem ...
func (s Sandstone) EncodeItem() (id int32, meta int16) {
	if s.Red {
		return 179, s.Data
	} else {
		return 24, s.Data
	}
}

// EncodeBlock ...
func (s Sandstone) EncodeBlock() (name string, properties map[string]interface{}) {
	name = "minecraft:sandstone"
	if s.Red {
		name = "minecraft:red_sandstone"
	}
	switch s.Data {
	case 0:
		properties = map[string]interface{}{"sand_stone_type": "default"}
	case 1:
		properties = map[string]interface{}{"sand_stone_type": "heiroglyphs"}
	case 2:
		properties = map[string]interface{}{"sand_stone_type": "cut"}
	case 3:
		properties = map[string]interface{}{"sand_stone_type": "smooth"}
	}
	return
}

// Hash ...
func (s Sandstone) Hash() uint64 {
	return hashSandstone | (uint64(boolByte(s.Red)) << 32) | (uint64(s.Data) << 33)
}

// allSandstone returns all possible sandstone states.
func allSandstone() (sandstones []world.Block) {
	for i := 0; i < 4; i++ {
		sandstones = append(sandstones, Sandstone{Red: false, Data: int16(i)})
		sandstones = append(sandstones, Sandstone{Red: true, Data: int16(i)})
	}
	return
}
