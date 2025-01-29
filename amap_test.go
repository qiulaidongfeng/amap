//go:build go1.24 && goexperiment.arenas

package amap

import (
	"strconv"
	"testing"

	"arena"
)

var a = arena.NewArena()

func arena_alloc[K comparable, V any](cap int) []group[K, V] {
	return arena.MakeSlice[group[K, V]](a, cap, cap)
}

func arena_alloc2[K comparable, V any]() (*arena.Arena, func(cap int) []group[K, V]) {
	a := arena.NewArena()
	return a, func(cap int) []group[K, V] { return arena.MakeSlice[group[K, V]](a, cap, cap) }
}

func TestMSetAndGet(t *testing.T) {
	m := NewM[uint64, uint64](arena_alloc, 1)
	m.Set(1, 2)
	v, ok := m.Get(1)
	if !ok {
		t.Fatal("got false, want true")
	}
	if v != 2 {
		t.Fatalf("got %d, want 1", v)
	}
}

func BenchmarkMSetAndGet(b *testing.B) {
	b.Run("uint", func(b *testing.B) {
		a, alloc := arena_alloc2[uint64, uint64]()
		defer a.Free()
		m := NewM[uint64, uint64](alloc, 1)
		for i := uint64(0); i < uint64(b.N); i++ {
			m.Set(i, 2)
			_, _ = m.Get(i)
		}
	})
	b.Run("string", func(b *testing.B) {
		a, alloc := arena_alloc2[string, string]()
		defer a.Free()
		m := NewM[string, string](alloc, 1)
		for i := 0; i < b.N; i++ {
			m.Set(strconv.Itoa(i), "2")
			_, _ = m.Get(strconv.Itoa(i))
		}
	})
}

func FuzzMSetAndGet(f *testing.F) {
	f.Fuzz(func(t *testing.T, count uint64) {
		testSetAndGet(t, count)
	})
}

func testMSetAndGet(t *testing.T, count uint64) {
	a, alloc := arena_alloc2[uint64, uint64]()
	defer a.Free()
	m := NewM[uint64, uint64](alloc, 1)
	for i := uint64(1); i < count; i++ {
		m.Set(i, i)
	}
	for i := uint64(1); i < count; i++ {
		i, ok := m.Get(i)
		if !ok {
			t.Fatal("got false, want true")
		}
		if i != i {
			t.Fatalf("got %d, want 1", i)
		}
	}
}

func TestMDel(t *testing.T) {
	m := NewM[uint64, uint64](arena_alloc, 1)
	m.Set(1, 2)
	m.Del(1)
	v, ok := m.Get(1)
	if ok {
		t.Fatal("got true, want false")
	}
	if v != 0 {
		t.Fatalf("got %d, want 0", v)
	}
}

func TestMClear(t *testing.T) {
	m := NewM[uint64, uint64](arena_alloc, 1)
	const count = 8192
	for i := uint64(1); i < count; i++ {
		m.Set(i, i)
	}
	m.Clear()
	got := 0
	m.Range(func(k, v uint64) bool {
		got++
		return true
	})
	if got != 0 {
		t.Fatalf("got %d, want 0", got)
	}
}
