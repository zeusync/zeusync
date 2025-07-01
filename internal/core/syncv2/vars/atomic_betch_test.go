package vars

import (
	sync "github.com/zeusync/zeusync/internal/core/syncv2"
	"math/rand"
	"testing"
	"unsafe"
)

type operationType int

const (
	OpSet operationType = iota
	OpGet
	OpSwap
)

const (
	Sync  = "Sync"
	Async = "Async"
)

func Benchmark_AtomicInt64(b *testing.B) {
	runAtomicBenchmarks(b,
		"AtomicInt64",
		NewAtomicInt64(0),
		func() int64 { return rand.Int63n(1_000) },
		1_000,
	)
}

func Benchmark_AtomicInt32(b *testing.B) {
	runAtomicBenchmarks(b,
		"AtomicInt32",
		NewAtomicInt32(0),
		func() int32 { return rand.Int31n(1_000) },
		1_000,
	)
}

func Benchmark_AtomicUint64(b *testing.B) {
	runAtomicBenchmarks(b,
		"AtomicUint64",
		NewAtomicUint64(0),
		func() uint64 { return rand.Uint64() },
		1_000,
	)
}

func Benchmark_AtomicUint32(b *testing.B) {
	runAtomicBenchmarks(b,
		"AtomicUint32",
		NewAtomicUint32(0),
		func() uint32 { return rand.Uint32() },
		1_000,
	)
}

func Benchmark_AtomicBool(b *testing.B) {
	runAtomicBenchmarks(b,
		"AtomicBool",
		NewAtomicBool(false),
		func() bool { return rand.Intn(2) == 1 },
		2,
	)
}

func Benchmark_AtomicString(b *testing.B) {
	strings := []string{"hello", "world", "test", "benchmark", "atomic"}
	runAtomicBenchmarks(b,
		"AtomicString",
		NewAtomicString(""),
		func() string { return strings[rand.Intn(len(strings))] },
		100,
	)
}

func Benchmark_AtomicStruct(b *testing.B) {
	type object struct {
		ID   int64
		Name string
		Tags []string
		Age  uint8
	}

	objects := []object{
		{
			ID:   1,
			Name: "name_1",
			Tags: []string{"tag_1", "tag_2", "tag_3"},
			Age:  20,
		},
		{
			ID:   2,
			Name: "name_2",
			Tags: []string{"tag_1", "tag_2", "tag_3"},
			Age:  30,
		},
		{
			ID:   3,
			Name: "name_3",
			Tags: []string{"tag_1", "tag_2", "tag_3"},
			Age:  40,
		},
	}
	runAtomicBenchmarks(b,
		"AtomicStruct",
		NewAtomicValue[object](objects[0]),
		func() object { return objects[rand.Intn(len(objects))] },
		100,
	)
}

func Benchmark_AtomicPointer(b *testing.B) {
	type object struct {
		ID   int64
		Name string
		Tags []string
		Age  uint8
	}

	pointers := []*object{
		{
			ID:   1,
			Name: "name_1",
			Tags: []string{"tag_1", "tag_2", "tag_3"},
			Age:  20,
		},
		{
			ID:   2,
			Name: "name_2",
			Tags: []string{"tag_1", "tag_2", "tag_3"},
			Age:  30,
		},
		{
			ID:   3,
			Name: "name_3",
			Tags: []string{"tag_1", "tag_2", "tag_3"},
			Age:  40,
		},
	}
	runAtomicBenchmarks(b,
		"AtomicStruct",
		NewAtomicPointer(unsafe.Pointer(pointers[0])),
		func() unsafe.Pointer { return unsafe.Pointer(pointers[rand.Intn(len(pointers))]) },
		100,
	)
}

func Benchmark_AtomicInt32_Reproducible(b *testing.B) {
	runReproducibleBenchmarks(b,
		"AtomicInt32",
		NewAtomicInt32(0),
		func(rng *rand.Rand) int32 { return rng.Int31n(1_000) },
		1_000,
		42,
	)
}

func runAtomicBenchmarks[T any](
	b *testing.B,
	name string,
	root sync.AtomicRoot[T],
	valueGenerator func() T,
	poolSize int,
) {
	values := generateValues(valueGenerator, poolSize)

	operations := []struct {
		name string
		op   operationType
	}{
		{"Set", OpSet},
		{"Get", OpGet},
		{"Swap", OpSwap},
	}

	modes := []string{Sync, Async}

	for _, operation := range operations {
		for _, mode := range modes {
			benchName := name + " | " + operation.name + " | " + mode
			b.Run(benchName, func(b *testing.B) {
				runBenchmark(b, root, values, operation.op, mode)
			})
		}
	}
}

func runBenchmark[T any](
	b *testing.B,
	root sync.AtomicRoot[T],
	values []T,
	operation operationType,
	mode string,
) {
	b.ReportAllocs()
	b.ResetTimer()

	valuesLen := len(values)

	if mode == Sync {
		switch operation {
		case OpSet:
			for i := 0; i < b.N; i++ {
				root.Set(values[i%valuesLen])
			}
		case OpGet:
			for i := 0; i < b.N; i++ {
				_ = root.Get()
			}
		case OpSwap:
			for i := 0; i < b.N; i++ {
				_ = root.Swap(values[i%valuesLen])
			}
		}
	} else {
		switch operation {
		case OpSet:
			b.RunParallel(func(pb *testing.PB) {
				i := 0
				for pb.Next() {
					root.Set(values[i%valuesLen])
					i++
				}
			})
		case OpGet:
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					_ = root.Get()
				}
			})
		case OpSwap:
			b.RunParallel(func(pb *testing.PB) {
				i := 0
				for pb.Next() {
					_ = root.Swap(values[i%valuesLen])
					i++
				}
			})
		}
	}
}

func generateValues[T any](generator func() T, size int) []T {
	values := make([]T, size)
	for i := 0; i < size; i++ {
		values[i] = generator()
	}
	return values
}

type BenchmarkConfig[T any] struct {
	Name      string
	Root      sync.AtomicRoot[T]
	Generator func() T
	PoolSize  int
}

func runBenchmarkConfig[T any](b *testing.B, config BenchmarkConfig[T]) {
	runAtomicBenchmarks(b, config.Name, config.Root, config.Generator, config.PoolSize)
}

func Benchmark_AllAtomics(b *testing.B) {
	b.Run("Int64", func(b *testing.B) {
		runBenchmarkConfig(b, BenchmarkConfig[int64]{
			Name:      "AtomicInt64",
			Root:      NewAtomicInt64(0),
			Generator: func() int64 { return rand.Int63n(1_000) },
			PoolSize:  1_000,
		})
	})

	b.Run("Int32", func(b *testing.B) {
		runBenchmarkConfig(b, BenchmarkConfig[int32]{
			Name:      "AtomicInt32",
			Root:      NewAtomicInt32(0),
			Generator: func() int32 { return rand.Int31n(1_000) },
			PoolSize:  1_000,
		})
	})

	b.Run("Bool", func(b *testing.B) {
		runBenchmarkConfig(b, BenchmarkConfig[bool]{
			Name:      "AtomicBool",
			Root:      NewAtomicBool(false),
			Generator: func() bool { return rand.Intn(2) == 1 },
			PoolSize:  2,
		})
	})
}

func runReproducibleBenchmarks[T any](
	b *testing.B,
	name string,
	root sync.AtomicRoot[T],
	valueGenerator func(*rand.Rand) T,
	poolSize int,
	seed int64,
) {
	rng := rand.New(rand.NewSource(seed))
	values := make([]T, poolSize)
	for i := 0; i < poolSize; i++ {
		values[i] = valueGenerator(rng)
	}

	operations := []struct {
		name string
		op   operationType
	}{
		{"Set", OpSet},
		{"Get", OpGet},
		{"Swap", OpSwap},
	}

	modes := []string{Sync, Async}

	for _, operation := range operations {
		for _, mode := range modes {
			benchName := name + " | " + operation.name + " | " + mode
			b.Run(benchName, func(b *testing.B) {
				runBenchmark(b, root, values, operation.op, mode)
			})
		}
	}
}
