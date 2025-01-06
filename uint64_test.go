package amap

import "testing"

func testalloc(cap int) []uint64 {
	return make([]uint64, cap)
}

func TestSetAndGet(t *testing.T) {
	m := NewUint64(testalloc, 1)
	m.Set(1, 2)
	v, ok := m.Get(1)
	if !ok {
		t.Fatal("got false, want true")
	}
	if v != 2 {
		t.Fatalf("got %d, want 1", v)
	}
}

func BenchmarkSetAndGet(b *testing.B) {
	m := NewUint64(testalloc, 1)
	for i := 0; i < b.N; i++ {
		m.Set(1, 2)
		_, _ = m.Get(1)
	}
}

func FuzzSetAndGet(f *testing.F) {
	f.Fuzz(func(t *testing.T, count uint64) {
		testSetAndGet(t, count)
	})
}

func testSetAndGet(t *testing.T, count uint64) {
	m := NewUint64(testalloc, 1)
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

func TestFail1(t *testing.T) {
	testSetAndGet(t, 32)
}
