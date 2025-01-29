//go:build go1.24

package amap

import "hash/maphash"

// M 类似一个map[K]V.
type M[K comparable, V any] struct {
	alloc func(int) []group[K, V]
	kv    []group[K, V]
	seed  maphash.Seed
	len   uint64
	cap   uint64
}

type group[K comparable, V any] struct {
	controlWord [groupsize / 2]uint64
	K           [groupsize / 2]K
	V           [groupsize / 2]V
	overflow    []group[K, V]
}

// NewM 创建一个 [M].
//   - alloc 分配指定数量的[group]，可以配合arena使用.
//   - mincap 指定最小的键加值容量.
func NewM[K comparable, V any](alloc func(int) []group[K, V], mincap int) *M[K, V] {
	ret := &M[K, V]{
		alloc: alloc,
		seed:  maphash.MakeSeed(),
	}
	mincap = align(mincap)
	//分配内存
	ret.kv = alloc(mincap)
	ret.cap = uint64(mincap) * keycap
	return ret
}

// align 将mincap对齐到groupsize的倍数
func align(mincap int) int {
	//确保mincap是groupsize的倍数
	if mincap == 0 {
		mincap = groupsize
	} else if mincap%groupsize != 0 {
		mincap += groupsize - (mincap % groupsize)
	}
	return mincap
}

// Set 类似 m[k]=v 对于go原生map.
func (m *M[K, V]) Set(k K, v V) {
	if m.set(k, v) {
		return
	}
	for {
		m.rehash()
		if m.set(k, v) {
			return
		}
	}
}

func (m *M[K, V]) hash(k K) (h uint64, g *group[K, V], index uint64) {
	h = maphash.Comparable(m.seed, k)
	//首先确定插入到那个组
	group_hash := h % uint64(len(m.kv))
	group := &m.kv[group_hash]
	//尝试插入组中的一个位置
	group_internal_hash := h % keycap
	return h, group, group_internal_hash
}

// set 尝试将键值对插入哈希表中.
func (m *M[K, V]) set(k K, v V) bool {
	h, group, group_internal_hash := m.hash(k)
	if m.try_set(k, v, group_internal_hash, h, group) {
		return true
	}
	//尝试插入组中的其他位置
	for i := uint64(0); i < keycap; i++ {
		if m.try_set(k, v, i, h, group) {
			return true
		}
	}
	// 尝试插入溢出组
	if group.overflow == nil {
		group.overflow = m.alloc(1)
	}
	for i := range group.overflow {
		group := &group.overflow[i]
		for j := uint64(0); j < keycap; j++ {
			if m.try_set(k, v, j, h, group) {
				return true
			}
		}
	}
	if m.len < m.cap {
		//扩容溢出组，插入
		old := group.overflow
		group.overflow = m.alloc(len(group.overflow) + 1)
		copy(group.overflow, old)
		group := &group.overflow[len(group.overflow)-1]
		m.try_set(k, v, 0, h, group)
		return true
	}
	return false
}

// try_set 尝试将键值对插入组中的一个位置.
func (m *M[K, V]) try_set(k K, v V, index, h uint64, group *group[K, V]) bool {
	//如果指定位置没有键值对
	if group.controlWord[index] == 0 {
		group.K[index] = k
		group.V[index] = v
		group.controlWord[index] = h
		m.len++
		return true
	} else if group.controlWord[index] == h { //如果指定位置是这个键
		group.K[index] = k
		group.V[index] = v
		m.len++
		return true
	}
	return false
}

// Range 迭代哈希表中的所有键值对.
func (m *M[K, V]) Range(yield func(k K, v V) bool) {
	//遍历所有组
	for i := range m.kv {
		//遍历一个组
		for j := range keycap {
			if m.kv[i].controlWord[j] != 0 && !yield(m.kv[i].K[j], m.kv[i].V[j]) { //如果有键值对且调用者要求停止
				return
			}
		}
		//遍历一个组的溢出组
		for j := range m.kv[i].overflow {
			group := &m.kv[i].overflow[j]
			for j := 0; j < keycap; j++ {
				if group.controlWord[j] != 0 && !yield(group.K[j], group.V[j]) { //如果有键值对且调用者要求停止
					return
				}
			}
		}
	}
}

// rehash 将哈希表扩容2倍.
func (m *M[K, V]) rehash() {
	newcap := len(m.kv) * 2
	nm := NewM(m.alloc, newcap)
	m.Range(func(k K, v V) bool {
		nm.Set(k, v)
		return true
	})
	*m = *nm
}

// Get 类似 m[k] 对于go原生map.
func (m *M[K, V]) Get(k K) (v V, ok bool) {
	h, group, group_internal_hash := m.hash(k)
	//尝试查找组中的一个位置
	if v, ok := m.try_get(k, group_internal_hash, h, group); ok {
		return v, ok
	}
	//尝试查找组中的其他位置
	for i := uint64(0); i < keycap; i++ {
		if v, ok := m.try_get(k, i, h, group); ok {
			return v, ok
		}
	}
	//尝试查找溢出组
	for i := range group.overflow {
		group := &group.overflow[i]
		for j := uint64(0); j < keycap; j++ {
			if v, ok := m.try_get(k, j, h, group); ok {
				return v, ok
			}
		}
	}
	return *new(V), false
}

// try_get 尝试从组中的一个位置获取指定键值对.
func (m *M[K, V]) try_get(k K, index, h uint64, group *group[K, V]) (v V, ok bool) {
	if group.controlWord[index] == h && group.K[index] == k { //如果指定位置包含指定的键值对
		return group.V[index], true
	}
	return *new(V), false
}

// Del 类似 delete(m,k) 对于go原生map.
// 但是并不立刻清除已删除的键值对
// 一个例子
// 对于 M[int,*int] ，调用Del意味着指针并不会立刻从哈希表中清除.
func (m *M[K, V]) Del(k K) {
	h, group, group_internal_hash := m.hash(k)
	//尝试从组中的一个位置删除
	if m.try_del(k, group_internal_hash, h, group, false) {
		return
	}
	//尝试从组中的其他位置删除
	for i := uint64(0); i < keycap; i++ {
		if m.try_del(k, i, h, group, false) {
			return
		}
	}
	//尝试从溢出组删除
	for i := range group.overflow {
		group := &group.overflow[i]
		for j := uint64(0); j < keycap; j++ {
			if m.try_del(k, j, h, group, false) {
				return
			}
		}
	}
}

// DelAndClear 类似 delete(m,k) 对于go原生map.
// 但是立刻清除已删除的键值对
// 一个例子
// 对于 M[int,*int] ，调用DelAndClear意味着指针会立刻从哈希表中清除.
func (m *M[K, V]) DelAndClear(k K) {
	h, group, group_internal_hash := m.hash(k)
	//尝试从组中的一个位置删除
	if m.try_del(k, group_internal_hash, h, group, true) {
		return
	}
	//尝试从组中的其他位置删除
	for i := uint64(0); i < keycap; i++ {
		if m.try_del(k, i, h, group, true) {
			return
		}
	}
	//尝试从溢出组删除
	for i := range group.overflow {
		group := &group.overflow[i]
		for j := uint64(0); j < keycap; j++ {
			if m.try_del(k, j, h, group, true) {
				return
			}
		}
	}
}

// try_del 尝试从组中的一个位置删除指定键值对.
func (m *M[K, V]) try_del(k K, index, h uint64, group *group[K, V], clear bool) (ok bool) {
	if group.controlWord[index] == h && group.K[index] == k { //如果指定位置包含指定的键值对
		group.controlWord[index] = 0
		if clear {
			group.K[index] = *new(K)
			group.V[index] = *new(V)
		}
		return true
	}
	return false
}

// Clear 类似 clear(m) 对于go原生map.
// 像go原生map那样，不收缩底层内存.
func (m *M[K, V]) Clear() {
	//遍历所有组
	for i := range m.kv {
		//遍历一个组
		for j := range keycap {
			m.kv[i].controlWord[j] = 0
			m.kv[i].K[j] = *new(K)
			m.kv[i].V[j] = *new(V)
		}
		for j := range m.kv[i].overflow {
			group := &m.kv[i].overflow[j]
			for j := range keycap {
				group.controlWord[j] = 0
				group.K[j] = *new(K)
				group.V[j] = *new(V)
			}
		}
	}
}
