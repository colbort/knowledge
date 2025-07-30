package main

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/bits-and-blooms/bitset"
)

func printMemStats(tag string) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Printf("[%s] Alloc = %v KB\n", tag, m.Alloc/1024)
}

func boolSliceTest(size int) {
	printMemStats("Before bool slice")
	bs := make([]bool, size)
	for i := 0; i < size; i++ {
		bs[i] = true
	}
	printMemStats("After bool slice")
}

func bitsetTest(size uint) {
	printMemStats("Before bitset")
	bs := bitset.New(size)
	for i := uint(0); i < size; i++ {
		bs.Set(i)
	}
	printMemStats("After bitset")
}

func TestBitset(t *testing.T) {
	size := 1_000_000 // 100万 bit
	fmt.Printf("\n🔍 Testing memory usage for %d bits:\n", size)

	fmt.Println("\n=== Bool Slice ===")
	boolSliceTest(size)

	runtime.GC() // 强制 GC

	fmt.Println("\n=== BitSet (uint64) ===")
	bitsetTest(uint(size))
}
