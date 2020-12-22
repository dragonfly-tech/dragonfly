package generator

import (
	"github.com/aquilax/go-perlin"
	"github.com/df-mc/dragonfly/dragonfly/block"
	"github.com/df-mc/dragonfly/dragonfly/block/wood"
	"github.com/df-mc/dragonfly/dragonfly/world"
	"github.com/df-mc/dragonfly/dragonfly/world/chunk"
	"math/rand"
)

var (
	stone, _ = world.BlockRuntimeID(block.Stone{})
)

type Vanilla struct {
	Smoothness, ForestSize, ChanceForTrees float64

	TerrainPerlin *perlin.Perlin
	TreesPerlin   *perlin.Perlin
}

var (
	log, _    = world.BlockRuntimeID(block.Log{Wood: wood.Oak()})
	leaves, _ = world.BlockRuntimeID(block.Leaves{Wood: wood.Oak()})
)

func NewVanillaGenerator(seed int64, alpha, beta, smoothness, forestsize, chancefortrees float64) (v Vanilla) {
	v.TerrainPerlin = perlin.NewPerlin(alpha, beta, 2, seed)
	v.TreesPerlin = perlin.NewPerlin(alpha, beta, 2, seed/2)
	v.ForestSize = forestsize
	v.Smoothness = smoothness
	v.ChanceForTrees = chancefortrees

	return
}

// GenerateChunk ...
func (v Vanilla) GenerateChunk(pos world.ChunkPos, chunk *chunk.Chunk) {
	for x := uint8(0); x < 16; x++ {
		for z := uint8(0); z < 16; z++ {
			chunk.SetRuntimeID(x, 0, z, 0, bedrock)
			max := uint8(52 + (v.TerrainPerlin.Noise2D(((16*(float64(pos.X())))+float64(x))/v.Smoothness, ((16*(float64(pos.Z())))+float64(z))/v.Smoothness) * 15))
			for y := uint8(1); y < max; y++ {
				chunk.SetRuntimeID(x, y, z, 0, stone)
			}
			chunk.SetRuntimeID(x, max+1, z, 0, dirt)
			chunk.SetRuntimeID(x, max+2, z, 0, dirt)
		}
	}
	v.GenerateTrees(pos, chunk)
}

func (v Vanilla) GrassLevel(x, z uint8, pos world.ChunkPos) uint8 {
	return uint8(52+(v.TerrainPerlin.Noise2D(((16*(float64(pos.X())))+float64(x))/v.Smoothness, ((16*(float64(pos.Z())))+float64(z))/v.Smoothness)*15)) + 2
}

func (v Vanilla) GenerateTrees(pos world.ChunkPos, chunk *chunk.Chunk) {
	for x := uint8(0); x < 16; x++ {
		for z := uint8(0); z < 16; z++ {
			chance := v.TreesPerlin.Noise2D((float64(pos.X())+float64(x))/v.ForestSize, (float64(pos.X())+float64(x))/v.ForestSize)

			if chance < v.ChanceForTrees {
				v.GenerateTree(x, v.GrassLevel(x, z, pos), z, chunk)
			}
		}
	}
	v.GenerateTrees(pos, chunk)
}

func (v Vanilla) GenerateTree(X, Y, Z uint8, chunk *chunk.Chunk) {
	var x, y, z uint8
	x = X % 16
	y = Y % 16
	z = Z % 16
	for y < y+3+uint8(rand.Intn(3)) {
		chunk.SetRuntimeID(x, y, z, 0, log)
		y++
	}
}
