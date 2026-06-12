package s2

import "sync"

// Table size constants
const (
	betterLongTableBits = 17
	betterLongTableSize = 1 << betterLongTableBits // 131072

	betterShortTableBits = 14
	betterShortTableSize = 1 << betterShortTableBits // 16384

	betterSnappyLongTableBits = 16
	betterSnappyLongTableSize = 1 << betterSnappyLongTableBits // 65536

	bestLongTableBits = 19
	bestLongTableSize = 1 << bestLongTableBits // 524288

	bestShortTableBits = 16
	bestShortTableSize = 1 << bestShortTableBits // 65536
)

type betterTables struct {
	lTable [betterLongTableSize]uint32
	sTable [betterShortTableSize]uint32
}

var betterTablePool = sync.Pool{New: func() interface{} { return &betterTables{} }}

// betterSnappyTables holds better-snappy compression hash tables.
type betterSnappyTables struct {
	lTable [betterSnappyLongTableSize]uint32
	sTable [betterShortTableSize]uint32
}

var betterSnappyTablePool = sync.Pool{New: func() interface{} { return &betterSnappyTables{} }}

// bestTables holds best compression hash tables.
type bestTables struct {
	lTable [bestLongTableSize]uint64
	sTable [bestShortTableSize]uint64
}

var bestTablePool = sync.Pool{New: func() interface{} { return &bestTables{} }}

// getBetterTables gets a zeroed betterTables from the pool.
func getBetterTables() *betterTables {
	t := betterTablePool.Get().(*betterTables)
	*t = betterTables{}
	return t
}

// getBetterSnappyTables gets a zeroed betterSnappyTables from the pool.
func getBetterSnappyTables() *betterSnappyTables {
	t := betterSnappyTablePool.Get().(*betterSnappyTables)
	*t = betterSnappyTables{}
	return t
}

// getBestTables gets a zeroed bestTables from the pool.
func getBestTables() *bestTables {
	t := bestTablePool.Get().(*bestTables)
	*t = bestTables{}
	return t
}
