package main

import (
	"flag"
	"fmt"
	"hash/fnv"
	"log"
	"math"
	"os"

	"github.com/bits-and-blooms/bitset"
)

type BloomFilter struct {
	bitset    *bitset.BitSet
	size      uint
	hashFuncs []func(string) uint
}

func generateHashFunc(seed uint32) func(string) uint {
	return func(s string) uint {
		h := fnv.New32a()
		h.Write([]byte(fmt.Sprintf("%d:%s", seed, s)))
		return uint(h.Sum32())
	}
}

func NewBloomFilter(n uint, p float64) *BloomFilter {
	m := optimalM(n, p)
	k := optimalK(m, n)
	funcs := make([]func(string) uint, k)
	for i := 0; i < int(k); i++ {
		funcs[i] = generateHashFunc(uint32(i))
	}
	return &BloomFilter{
		bitset:    bitset.New(m),
		size:      m,
		hashFuncs: funcs,
	}
}

func (bf *BloomFilter) Add(item string) {
	for _, hashFunc := range bf.hashFuncs {
		index := hashFunc(item) % bf.size
		bf.bitset.Set(index)
	}
}

func (bf *BloomFilter) Exists(item string) bool {
	for _, hashFunc := range bf.hashFuncs {
		index := hashFunc(item) % bf.size
		if !bf.bitset.Test(index) {
			return false
		}
	}
	return true
}

func optimalK(m, n uint) uint {
	return uint(math.Round((float64(m) / float64(n)) * math.Ln2))
}

func optimalM(n uint, p float64) uint {
	return uint(math.Ceil(float64(n) * math.Log(1/p) / (math.Pow(math.Ln2, 2))))
}

func estimateFalsePositiveRate(m, n, k uint) float64 {
	exponent := -1 * float64(k) * float64(n) / float64(m)
	return math.Pow(1-math.Exp(exponent), float64(k))
}

func main() {
	n := flag.Uint("n", 1000, "Expected number of elements")
	p := flag.Float64("p", 0.01, "Desired false positive rate (e.g. 0.01 for 1%)")
	item := flag.String("item", "", "Item to check")
	add := flag.Bool("add", false, "Whether to add the item")
	flag.Parse()

	if *n == 0 || *p <= 0 || *p >= 1 {
		log.Fatalf("[ERROR] Invalid parameters: n must be > 0 and 0 < p < 1")
	}

	bf := NewBloomFilter(*n, *p)
	k := uint(len(bf.hashFuncs))
	fmt.Printf("[INFO] Calculated m = %d, k = %d\n", bf.size, k)
	fmt.Printf("[INFO] Estimated False Positive Rate ≈ %.6f (%.2f%%)\n", estimateFalsePositiveRate(bf.size, *n, k), estimateFalsePositiveRate(bf.size, *n, k)*100)

	if *item != "" {
		if *add {
			bf.Add(*item)
			fmt.Printf("[ADD] Item '%s' added to Bloom filter.\n", *item)
		} else {
			exists := bf.Exists(*item)
			fmt.Printf("[CHECK] Item '%s' exists? %v\n", *item, exists)
		}
	} else {
		fmt.Println("[WARN] No item specified. Use -item to test.")
		flag.Usage()
		os.Exit(1)
	}
}
