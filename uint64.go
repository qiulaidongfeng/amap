package amap

import "unsafe"

// Uint64 类似一个map[uint64]uint64.
type Uint64 struct {
	alloc func(int) []uint64
	kv    [][groupsize]uint64
}

// groupsize 是自定义哈希表中一个组的键加值的数量.
const groupsize = 16

// NewUint64 创建一个 [Uint64].
//   - alloc 分配指定大小的内存，可以配合arena使用.
//   - mincap 指定最小的键加值容量.
func NewUint64(alloc func(int) []uint64, mincap int) *Uint64 {
	ret := &Uint64{
		alloc: alloc,
	}
	//确保mincap是groupsize的倍数
	if mincap == 0 {
		mincap = groupsize
	} else if mincap%groupsize != 0 {
		mincap += groupsize - (mincap % groupsize)
	}
	//分配内存
	mem := alloc(mincap)
	//转换为n个键值对组
	ret.kv = to_kv(mem)
	return ret
}

// to_kv 将一个[]uint64,转换为n个键值对组
// 假定切片长度是groupsize的倍数
func to_kv(mem []uint64) [][groupsize]uint64 {
	l := len(mem) / groupsize
	type slice struct {
		ptr      unsafe.Pointer
		len, cap int
	}
	r := slice{ptr: unsafe.Pointer(&mem[0]), len: l, cap: l}
	return *(*[][groupsize]uint64)(unsafe.Pointer(&r))
}

// Set 类似 m[k]=v 对于go原生map.
// k不能等于0.
func (m *Uint64) Set(k, v uint64) {
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

// set 尝试将键值对插入哈希表中.
func (m *Uint64) set(k, v uint64) bool {
	if k == 0 {
		panic("键不能为0")
	}
	//首先确定插入到那个组
	group_hash := k % uint64(len(m.kv))
	group := &m.kv[group_hash]
	//尝试插入组中的一个位置
	group_intrtnal_hash := k % 8
	if m.try_set(k, v, group_intrtnal_hash, group) {
		return true
	}
	//尝试插入组中的其他位置
	for i := uint64(0); i < 8; i++ {
		if m.try_set(k, v, i, group) {
			return true
		}
	}
	return false
}

// try_set 尝试将键值对插入组中的一个位置.
func (m *Uint64) try_set(k, v, index uint64, group *[16]uint64) bool {
	ki := index * 2
	kp := &group[ki]
	//如果指定位置没有键值对
	if *kp == 0 {
		*kp = k
		group[ki+1] = v
		return true
	} else if *kp == k { //如果指定位置是这个键
		group[ki+1] = v
		return true
	}
	return false
}

// Range 迭代哈希表中的所有键值对.
func (m *Uint64) Range(yield func(k, v uint64) bool) {
	//遍历所有组
	for i := range m.kv {
		//遍历一个组
		for k := 0; k < 16; k += 2 {
			if m.kv[i][k] != 0 && !yield(m.kv[i][k], m.kv[i][k+1]) { //如果有键值对且调用者要求停止
				return
			}
		}
	}
}

// rehash 将哈希表扩容2倍.
func (m *Uint64) rehash() {
	newcap := len(m.kv) * 2
	nm := NewUint64(m.alloc, newcap*groupsize)
	m.Range(func(k, v uint64) bool {
		nm.Set(k, v)
		return true
	})
	*m = *nm
}

// Get 类似 m[k] 对于go原生map.
// k不能等于0.
func (m *Uint64) Get(k uint64) (v uint64, ok bool) {
	if k == 0 {
		panic("键不能为0")
	}
	//首先确定要查找的那个组
	group_hash := k % uint64(len(m.kv))
	group := &m.kv[group_hash]
	group_intrtnal_hash := k % 8
	//尝试查找组中的一个位置
	if v, ok := m.try_get(k, group_intrtnal_hash, group); ok {
		return v, ok
	}
	//尝试查找组中的其他位置
	for i := uint64(0); i < 8; i++ {
		if v, ok := m.try_get(k, i, group); ok {
			return v, ok
		}
	}
	return 0, false
}

// try_get 尝试从组中的一个位置获取指定键值对.
func (m *Uint64) try_get(k, index uint64, group *[16]uint64) (v uint64, ok bool) {
	ki := index * 2
	kp := group[ki]
	if kp == k { //如果指定位置保护指定的键值对
		return group[ki+1], true
	}
	return 0, false
}
