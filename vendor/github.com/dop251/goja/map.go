package goja

import (
	"hash/maphash"
)

type mapEntry struct {
	key, value Value

	iterPrev, iterNext *mapEntry
	hNext              *mapEntry
}

type orderedMap struct {
	hash                *maphash.Hash
	hashTable           map[uint64]*mapEntry
	iterFirst, iterLast *mapEntry
	size                int
}

type orderedMapIter struct {
	m   *orderedMap
	cur *mapEntry
}

func (m *orderedMap) lookup(key Value) (h uint64, entry, hPrev *mapEntry) {
	if key == _negativeZero {
		key = intToValue(0)
	}
	h = key.hash(m.hash)
	for entry = m.hashTable[h]; entry != nil && !entry.key.SameAs(key); hPrev, entry = entry, entry.hNext {
	}
	return
}

func (m *orderedMap) set(key, value Value) {
	h, entry, hPrev := m.lookup(key)
	if entry != nil {
		entry.value = value
	} else {
		if key == _negativeZero {
			key = intToValue(0)
		}
		entry = &mapEntry{key: key, value: value}
		if hPrev == nil {
			m.hashTable[h] = entry
		} else {
			hPrev.hNext = entry
		}
		if m.iterLast != nil {
			entry.iterPrev = m.iterLast
			m.iterLast.iterNext = entry
		} else {
			m.iterFirst = entry
		}
		m.iterLast = entry
		m.size++
	}
}

func (m *orderedMap) get(key Value) Value {
	_, entry, _ := m.lookup(key)
	if entry != nil {
		return entry.value
	}

	return nil
}

func (m *orderedMap) remove(key Value) bool {
	h, entry, hPrev := m.lookup(key)
	if entry != nil {
		entry.key = nil
		entry.value = nil

		// remove from the doubly-linked list
		if entry.iterPrev != nil {
			entry.iterPrev.iterNext = entry.iterNext
		} else {
			m.iterFirst = entry.iterNext
		}
		if entry.iterNext != nil {
			entry.iterNext.iterPrev = entry.iterPrev
		} else {
			m.iterLast = entry.iterPrev
		}

		// remove from the hashTable
		if hPrev == nil {
			if entry.hNext == nil {
				delete(m.hashTable, h)
			} else {
				m.hashTable[h] = entry.hNext
			}
		} else {
			hPrev.hNext = entry.hNext
		}

		m.size--
		return true
	}

	return false
}

func (m *orderedMap) has(key Value) bool {
	_, entry, _ := m.lookup(key)
	return entry != nil
}

func (iter *orderedMapIter) next() *mapEntry {
	if iter.m == nil {
		// closed iterator
		return nil
	}

	cur := iter.cur
	// if the current item was deleted, track back to find the latest that wasn't
	for cur != nil && cur.key == nil {
		cur = cur.iterPrev
	}

	if cur != nil {
		cur = cur.iterNext
	} else {
		cur = iter.m.iterFirst
	}

	if cur == nil {
		iter.close()
	} else {
		iter.cur = cur
	}

	return cur
}

func (iter *orderedMapIter) close() {
	iter.m = nil
	iter.cur = nil
}

func newOrderedMap(h *maphash.Hash) *orderedMap {
	return &orderedMap{
		hash:      h,
		hashTable: make(map[uint64]*mapEntry),
	}
}

func (m *orderedMap) newIter() *orderedMapIter {
	iter := &orderedMapIter{
		m: m,
	}
	return iter
}

func (m *orderedMap) clear() {
	for item := m.iterFirst; item != nil; item = item.iterNext {
		item.key = nil
		item.value = nil
		if item.iterPrev != nil {
			item.iterPrev.iterNext = nil
		}
	}
	m.iterFirst = nil
	m.iterLast = nil
	m.hashTable = make(map[uint64]*mapEntry)
	m.size = 0
}
