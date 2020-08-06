package block

import (
	"github.com/df-mc/dragonfly/dragonfly/block/colour"
	"github.com/df-mc/dragonfly/dragonfly/block/fire"
	"github.com/df-mc/dragonfly/dragonfly/block/wood"
	"github.com/df-mc/dragonfly/dragonfly/internal/item_internal"
	"github.com/df-mc/dragonfly/dragonfly/world"
	_ "unsafe" // Imported for compiler directives.
)

// init registers all blocks implemented by Dragonfly.
func init() {
	// Always register Air first so we can use 0 runtime IDs as air.
	world.RegisterBlock(Air{})

	world.RegisterBlock(Stone{})
	world.RegisterBlock(Granite{}, Granite{Polished: true})
	world.RegisterBlock(Diorite{}, Diorite{Polished: true})
	world.RegisterBlock(Andesite{}, Andesite{Polished: true})
	world.RegisterBlock(Grass{}, GrassPath{})
	world.RegisterBlock(Dirt{}, Dirt{Coarse: true})
	world.RegisterBlock(Cobblestone{}, Cobblestone{Mossy: true})
	world.RegisterBlock(allKelp()...)
	world.RegisterBlock(allLogs()...)
	world.RegisterBlock(allLeaves()...)
	world.RegisterBlock(Bedrock{}, Bedrock{InfiniteBurning: true})
	world.RegisterBlock(Chest{Facing: world.East}, Chest{Facing: world.West}, Chest{Facing: world.North}, Chest{Facing: world.South})
	world.RegisterBlock(allConcrete()...)
	world.RegisterBlock(allLight()...)
	world.RegisterBlock(allPlanks()...)
	world.RegisterBlock(allWoodStairs()...)
	world.RegisterBlock(allWoodSlabs()...)
	world.RegisterBlock(allWater()...)
	world.RegisterBlock(allLava()...)
	world.RegisterBlock(Obsidian{})
	world.RegisterBlock(DiamondBlock{})
	world.RegisterBlock(Glass{})
	world.RegisterBlock(Glowstone{})
	world.RegisterBlock(EmeraldBlock{})
	world.RegisterBlock(EndBricks{})
	world.RegisterBlock(allEndBrickStairs()...)
	world.RegisterBlock(GoldBlock{})
	world.RegisterBlock(NetheriteBlock{})
	world.RegisterBlock(IronBlock{})
	world.RegisterBlock(CoalBlock{})
	world.RegisterBlock(Beacon{})
	world.RegisterBlock(Sponge{})
	world.RegisterBlock(Sponge{Wet: true})
	world.RegisterBlock(LapisBlock{})
	world.RegisterBlock(allStainedTerracotta()...)
	world.RegisterBlock(allGlazedTerracotta()...)
	world.RegisterBlock(Terracotta{})
	world.RegisterBlock(allStainedGlass()...)
	world.RegisterBlock(allStainedGlassPane()...)
	world.RegisterBlock(GlassPane{})
	world.RegisterBlock(IronBars{})
	world.RegisterBlock(allCarpets()...)
	world.RegisterBlock(allWool()...)
	world.RegisterBlock(allFenceGates()...)
	world.RegisterBlock(allTrapdoors()...)
	world.RegisterBlock(allDoors()...)
	world.RegisterBlock(allCoral()...)
	world.RegisterBlock(allCoralBlocks()...)
	world.RegisterBlock(allPumpkins()...)
	world.RegisterBlock(LitPumpkin{Facing: world.East}, LitPumpkin{Facing: world.West}, LitPumpkin{Facing: world.North}, LitPumpkin{Facing: world.South})
	world.RegisterBlock(EndStone{})
	world.RegisterBlock(Netherrack{})
	world.RegisterBlock(Clay{})
	world.RegisterBlock(BoneBlock{Axis: world.X}, BoneBlock{Axis: world.Y}, BoneBlock{Axis: world.Z})
	world.RegisterBlock(Lantern{Type: fire.Normal()}, Lantern{Type: fire.Normal(), Hanging: true}, Lantern{Type: fire.Soul()}, Lantern{Type: fire.Soul(), Hanging: true})
	world.RegisterBlock(AncientDebris{})
	world.RegisterBlock(EmeraldOre{})
	world.RegisterBlock(DiamondOre{})
	world.RegisterBlock(LapisOre{})
	world.RegisterBlock(NetherGoldOre{})
	world.RegisterBlock(GoldOre{})
	world.RegisterBlock(IronOre{})
	world.RegisterBlock(CoalOre{})
	world.RegisterBlock(allCocoaBeans()...)
	world.RegisterBlock(allSmoothStoneSlabs()...)
	world.RegisterBlock(allSandstoneSlabs()...)
	world.RegisterBlock(allCobblestoneSlabs()...)
	world.RegisterBlock(allWoodenSlabs()...)
	world.RegisterBlock(allBrickSlabs()...)
	world.RegisterBlock(allStoneBrickSlabs()...)
	world.RegisterBlock(allQuartzSlabs()...)
	world.RegisterBlock(allNetherBrickSlabs()...)
	world.RegisterBlock(allRedSandstoneSlabs()...)
	world.RegisterBlock(allPurpurSlabs()...)
	world.RegisterBlock(allPrismarineRoughSlabs()...)
	world.RegisterBlock(allPrismarineDarkSlabs()...)
	world.RegisterBlock(allPrismarineBrickSlabs()...)
	world.RegisterBlock(allMossyCobblestoneSlabs()...)
	world.RegisterBlock(allSmoothSandstoneSlabs()...)
	world.RegisterBlock(allRedNetherBrickSlabs()...)
	world.RegisterBlock(allEndStoneBrickSlabs()...)
	world.RegisterBlock(allSmoothRedSandstoneSlabs()...)
	world.RegisterBlock(allPolishedAndesiteSlabs()...)
	world.RegisterBlock(allAndesiteSlabs()...)
	world.RegisterBlock(allDioriteSlabs()...)
	world.RegisterBlock(allPolishedDioriteSlabs()...)
	world.RegisterBlock(allGraniteSlabs()...)
	world.RegisterBlock(allPolishedGraniteSlabs()...)
	world.RegisterBlock(allMossyStoneBrickSlabs()...)
	world.RegisterBlock(allSmoothQuartzSlabs()...)
	world.RegisterBlock(allStoneSlabs()...)
	world.RegisterBlock(allCutSandstoneSlabs()...)
	world.RegisterBlock(allCutRedSandstoneSlabs()...)
	world.RegisterBlock(allBlackstoneSlabs()...)
	world.RegisterBlock(allPolishedBlackstoneSlabs()...)
	world.RegisterBlock(allPolishedBlackstoneBrickSlabs()...)
}

func init() {
	world.RegisterItem("minecraft:air", Air{})
	world.RegisterItem("minecraft:stone", Stone{})
	world.RegisterItem("minecraft:stone", Granite{})
	world.RegisterItem("minecraft:stone", Granite{Polished: true})
	world.RegisterItem("minecraft:stone", Diorite{})
	world.RegisterItem("minecraft:stone", Diorite{Polished: true})
	world.RegisterItem("minecraft:stone", Andesite{})
	world.RegisterItem("minecraft:stone", Andesite{Polished: true})
	world.RegisterItem("minecraft:grass", Grass{})
	world.RegisterItem("minecraft:grass_path", GrassPath{})
	world.RegisterItem("minecraft:dirt", Dirt{})
	world.RegisterItem("minecraft:dirt", Dirt{Coarse: true})
	world.RegisterItem("minecraft:cobblestone", Cobblestone{})
	world.RegisterItem("minecraft:bedrock", Bedrock{})
	world.RegisterItem("minecraft:kelp", Kelp{})
	world.RegisterItem("minecraft:log", Log{Wood: wood.Oak()})
	world.RegisterItem("minecraft:log", Log{Wood: wood.Spruce()})
	world.RegisterItem("minecraft:log", Log{Wood: wood.Birch()})
	world.RegisterItem("minecraft:log", Log{Wood: wood.Jungle()})
	world.RegisterItem("minecraft:leaves", Leaves{Wood: wood.Oak()})
	world.RegisterItem("minecraft:leaves", Leaves{Wood: wood.Spruce()})
	world.RegisterItem("minecraft:leaves", Leaves{Wood: wood.Birch()})
	world.RegisterItem("minecraft:leaves", Leaves{Wood: wood.Jungle()})
	world.RegisterItem("minecraft:chest", Chest{})
	world.RegisterItem("minecraft:mossy_cobblestone", Cobblestone{Mossy: true})
	world.RegisterItem("minecraft:leaves2", Leaves{Wood: wood.Acacia()})
	world.RegisterItem("minecraft:leaves2", Leaves{Wood: wood.DarkOak()})
	world.RegisterItem("minecraft:log2", Log{Wood: wood.Acacia()})
	world.RegisterItem("minecraft:log2", Log{Wood: wood.DarkOak()})
	world.RegisterItem("minecraft:stripped_spruce_log", Log{Wood: wood.Spruce(), Stripped: true})
	world.RegisterItem("minecraft:stripped_birch_log", Log{Wood: wood.Birch(), Stripped: true})
	world.RegisterItem("minecraft:stripped_jungle_log", Log{Wood: wood.Jungle(), Stripped: true})
	world.RegisterItem("minecraft:stripped_acacia_log", Log{Wood: wood.Acacia(), Stripped: true})
	world.RegisterItem("minecraft:stripped_dark_oak_log", Log{Wood: wood.DarkOak(), Stripped: true})
	world.RegisterItem("minecraft:stripped_oak_log", Log{Wood: wood.Oak(), Stripped: true})
	for _, c := range colour.All() {
		world.RegisterItem("minecraft:concrete", Concrete{Colour: c})
		world.RegisterItem("minecraft:stained_hardened_clay", StainedTerracotta{Colour: c})
		world.RegisterItem("minecraft:carpet", Carpet{Colour: c})
		world.RegisterItem("minecraft:wool", Wool{Colour: c})
		world.RegisterItem("minecraft:stained_glass", StainedGlass{Colour: c})
		world.RegisterItem("minecraft:stained_glass_pane", StainedGlassPane{Colour: c})

		colourName := c.String()
		if c == colour.LightGrey() {
			colourName = "silver"
		}

		world.RegisterItem("minecraft:"+colourName+"_glazed_terracotta", GlazedTerracotta{Colour: c})
	}
	for _, b := range allLight() {
		world.RegisterItem("minecraft:light_block", b.(world.Item))
	}
	for _, b := range allPlanks() {
		world.RegisterItem("minecraft:planks", b.(world.Item))
	}
	world.RegisterItem("minecraft:oak_stairs", WoodStairs{Wood: wood.Oak()})
	world.RegisterItem("minecraft:spruce_stairs", WoodStairs{Wood: wood.Spruce()})
	world.RegisterItem("minecraft:birch_stairs", WoodStairs{Wood: wood.Birch()})
	world.RegisterItem("minecraft:jungle_stairs", WoodStairs{Wood: wood.Jungle()})
	world.RegisterItem("minecraft:acacia_stairs", WoodStairs{Wood: wood.Acacia()})
	world.RegisterItem("minecraft:dark_oak_stairs", WoodStairs{Wood: wood.DarkOak()})
	world.RegisterItem("minecraft:wooden_slab", WoodSlab{Wood: wood.Oak()})
	world.RegisterItem("minecraft:wooden_slab", WoodSlab{Wood: wood.Spruce()})
	world.RegisterItem("minecraft:wooden_slab", WoodSlab{Wood: wood.Birch()})
	world.RegisterItem("minecraft:wooden_slab", WoodSlab{Wood: wood.Jungle()})
	world.RegisterItem("minecraft:wooden_slab", WoodSlab{Wood: wood.Acacia()})
	world.RegisterItem("minecraft:wooden_slab", WoodSlab{Wood: wood.DarkOak()})
	world.RegisterItem("minecraft:double_wooden_slab", WoodSlab{Wood: wood.Oak(), Double: true})
	world.RegisterItem("minecraft:double_wooden_slab", WoodSlab{Wood: wood.Spruce(), Double: true})
	world.RegisterItem("minecraft:double_wooden_slab", WoodSlab{Wood: wood.Birch(), Double: true})
	world.RegisterItem("minecraft:double_wooden_slab", WoodSlab{Wood: wood.Jungle(), Double: true})
	world.RegisterItem("minecraft:double_wooden_slab", WoodSlab{Wood: wood.Acacia(), Double: true})
	world.RegisterItem("minecraft:double_wooden_slab", WoodSlab{Wood: wood.DarkOak(), Double: true})
	world.RegisterItem("minecraft:obsidian", Obsidian{})
	world.RegisterItem("minecraft:diamond_block", DiamondBlock{})
	world.RegisterItem("minecraft:glass", Glass{})
	world.RegisterItem("minecraft:glowstone", Glowstone{})
	world.RegisterItem("minecraft:emerald_block", EmeraldBlock{})
	world.RegisterItem("minecraft:end_bricks", EndBricks{})
	world.RegisterItem("minecraft:end_brick_stairs", EndBrickStairs{})
	world.RegisterItem("minecraft:netherite_block", NetheriteBlock{})
	world.RegisterItem("minecraft:gold_block", GoldBlock{})
	world.RegisterItem("minecraft:iron_block", IronBlock{})
	world.RegisterItem("minecraft:coal_block", CoalBlock{})
	world.RegisterItem("minecraft:beacon", Beacon{})
	world.RegisterItem("minecraft:sponge", Sponge{})
	world.RegisterItem("minecraft:wet_sponge", Sponge{Wet: true})
	world.RegisterItem("minecraft:lapis_block", LapisBlock{})
	world.RegisterItem("minecraft:hardened_clay", Terracotta{})
	world.RegisterItem("minecraft:glass_pane", GlassPane{})
	world.RegisterItem("minecraft:iron_bars", IronBars{})
	world.RegisterItem("minecraft:fence_gate", WoodFenceGate{Wood: wood.Oak()})
	world.RegisterItem("minecraft:spruce_fence_gate", WoodFenceGate{Wood: wood.Spruce()})
	world.RegisterItem("minecraft:birch_fence_gate", WoodFenceGate{Wood: wood.Birch()})
	world.RegisterItem("minecraft:jungle_fence_gate", WoodFenceGate{Wood: wood.Jungle()})
	world.RegisterItem("minecraft:acacia_fence_gate", WoodFenceGate{Wood: wood.Acacia()})
	world.RegisterItem("minecraft:dark_oak_fence_gate", WoodFenceGate{Wood: wood.DarkOak()})
	world.RegisterItem("minecraft:wooden_trapdoor", WoodTrapdoor{Wood: wood.Oak()})
	world.RegisterItem("minecraft:spruce_trapdoor", WoodTrapdoor{Wood: wood.Spruce()})
	world.RegisterItem("minecraft:birch_trapdoor", WoodTrapdoor{Wood: wood.Birch()})
	world.RegisterItem("minecraft:jungle_trapdoor", WoodTrapdoor{Wood: wood.Jungle()})
	world.RegisterItem("minecraft:acacia_trapdoor", WoodTrapdoor{Wood: wood.Acacia()})
	world.RegisterItem("minecraft:dark_oak_trapdoor", WoodTrapdoor{Wood: wood.DarkOak()})
	world.RegisterItem("minecraft:wooden_door", WoodDoor{Wood: wood.Oak()})
	world.RegisterItem("minecraft:spruce_door", WoodDoor{Wood: wood.Spruce()})
	world.RegisterItem("minecraft:birch_door", WoodDoor{Wood: wood.Birch()})
	world.RegisterItem("minecraft:jungle_door", WoodDoor{Wood: wood.Jungle()})
	world.RegisterItem("minecraft:acacia_door", WoodDoor{Wood: wood.Acacia()})
	world.RegisterItem("minecraft:dark_oak_door", WoodDoor{Wood: wood.DarkOak()})
	for _, c := range allCoral() {
		world.RegisterItem("minecraft:coral", c.(world.Item))
	}
	for _, c := range allCoralBlocks() {
		world.RegisterItem("minecraft:coral_block", c.(world.Item))
	}
	world.RegisterItem("minecraft:pumpkin", Pumpkin{})
	world.RegisterItem("minecraft:lit_pumpkin", LitPumpkin{})
	world.RegisterItem("minecraft:carved_pumpkin", Pumpkin{Carved: true})
	world.RegisterItem("minecraft:end_stone", EndStone{})
	world.RegisterItem("minecraft:netherrack", Netherrack{})
	world.RegisterItem("minecraft:clay", Clay{})
	world.RegisterItem("minecraft:bone_block", BoneBlock{})
	world.RegisterItem("minecraft:lantern", Lantern{Type: fire.Normal()})
	world.RegisterItem("minecraft:soul_lantern", Lantern{Type: fire.Soul()})
	world.RegisterItem("minecraft:ancient_debris", AncientDebris{})
	world.RegisterItem("minecraft:emerald_ore", EmeraldOre{})
	world.RegisterItem("minecraft:diamond_ore", DiamondOre{})
	world.RegisterItem("minecraft:lapis_ore", LapisOre{})
	world.RegisterItem("minecraft:nether_gold_ore", NetherGoldOre{})
	world.RegisterItem("minecraft:gold_ore", GoldOre{})
	world.RegisterItem("minecraft:iron_ore", IronOre{})
	world.RegisterItem("minecraft:coal_ore", CoalOre{})
	world.RegisterItem("minecraft:dye", CocoaBean{})
	world.RegisterItem("minecraft:stone_slab", SmoothStoneSlab{})
	world.RegisterItem("minecraft:stone_slab", SandstoneSlab{})
	world.RegisterItem("minecraft:stone_slab", WoodenSlab{})
	world.RegisterItem("minecraft:stone_slab", CobblestoneSlab{})
	world.RegisterItem("minecraft:stone_slab", BrickSlab{})
	world.RegisterItem("minecraft:stone_slab", StoneBrickSlab{})
	world.RegisterItem("minecraft:stone_slab", QuartzSlab{})
	world.RegisterItem("minecraft:stone_slab", NetherBrickSlab{})
	world.RegisterItem("minecraft:stone_slab2", RedSandstoneSlab{})
	world.RegisterItem("minecraft:stone_slab2", PurpurSlab{})
	world.RegisterItem("minecraft:stone_slab2", PrismarineRoughSlab{})
	world.RegisterItem("minecraft:stone_slab2", PrismarineDarkSlab{})
	world.RegisterItem("minecraft:stone_slab2", PrismarineBrickSlab{})
	world.RegisterItem("minecraft:stone_slab2", MossyCobblestoneSlab{})
	world.RegisterItem("minecraft:stone_slab2", SmoothSandstoneSlab{})
	world.RegisterItem("minecraft:stone_slab2", RedNetherBrickSlab{})
	world.RegisterItem("minecraft:stone_slab3", EndStoneBrickSlab{})
	world.RegisterItem("minecraft:stone_slab3", SmoothRedSandstoneSlab{})
	world.RegisterItem("minecraft:stone_slab3", PolishedAndesiteSlab{})
	world.RegisterItem("minecraft:stone_slab3", AndesiteSlab{})
	world.RegisterItem("minecraft:stone_slab3", DioriteSlab{})
	world.RegisterItem("minecraft:stone_slab3", PolishedDioriteSlab{})
	world.RegisterItem("minecraft:stone_slab3", GraniteSlab{})
	world.RegisterItem("minecraft:stone_slab3", PolishedGraniteSlab{})
	world.RegisterItem("minecraft:stone_slab4", MossyStoneBrickSlab{})
	world.RegisterItem("minecraft:stone_slab4", SmoothQuartzSlab{})
	world.RegisterItem("minecraft:stone_slab4", StoneSlab{})
	world.RegisterItem("minecraft:stone_slab4", CutSandstoneSlab{})
	world.RegisterItem("minecraft:stone_slab4", CutRedSandstoneSlab{})
	world.RegisterItem("minecraft:blackstone_slab", BlackstoneSlab{})
	world.RegisterItem("minecraft:polished_blackstone_slab", PolishedBlackstoneSlab{})
	world.RegisterItem("minecraft:polished_blackstone_brick_slab", PolishedBlackstoneBrickSlab{})
}

func init() {
	item_internal.Air = Air{}
	item_internal.Grass = Grass{}
	item_internal.GrassPath = GrassPath{}
	item_internal.IsUnstrippedLog = func(b world.Block) bool {
		l, ok := b.(Log)
		return ok && !l.Stripped
	}
	item_internal.StripLog = func(b world.Block) world.Block {
		l := b.(Log)
		l.Stripped = true
		return l
	}
	item_internal.IsCarvedPumpkin = func(b world.Item) bool {
		p, ok := b.(Pumpkin)
		return ok && p.Carved
	}
	item_internal.IsUncarvedPumpkin = func(b world.Block) bool {
		p, ok := b.(Pumpkin)
		return ok && !p.Carved
	}
	item_internal.CarvePumpkin = func(b world.Block, face world.Face) world.Block {
		return Pumpkin{Carved: true, Facing: face.Direction()}
	}
	item_internal.Lava = Lava{Depth: 8, Still: true}
	item_internal.Water = Water{Depth: 8, Still: true}
	item_internal.IsWater = func(b world.Liquid) bool {
		_, ok := b.(Water)
		return ok
	}
	item_internal.Replaceable = replaceableWith
}

// readSlice reads an interface slice from a map at the key passed.
//noinspection GoCommentLeadingSpace
func readSlice(m map[string]interface{}, key string) []interface{} {
	//lint:ignore S1005 Double assignment is done explicitly to prevent panics.
	v, _ := m[key]
	b, _ := v.([]interface{})
	return b
}

// readString reads a string from a map at the key passed.
//noinspection GoCommentLeadingSpace
func readString(m map[string]interface{}, key string) string {
	//lint:ignore S1005 Double assignment is done explicitly to prevent panics.
	v, _ := m[key]
	b, _ := v.(string)
	return b
}

// readInt32 reads an int32 from a map at the key passed.
//noinspection GoCommentLeadingSpace
func readInt32(m map[string]interface{}, key string) int32 {
	//lint:ignore S1005 Double assignment is done explicitly to prevent panics.
	v, _ := m[key]
	b, _ := v.(int32)
	return b
}
