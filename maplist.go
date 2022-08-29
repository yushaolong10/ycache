package ycache

import (
	"container/list"
	"sync"
)

type simpleMapList struct {
	mutex   sync.Mutex
	ids     map[string]struct{}
	mapList map[string]*mapEntry

	valueCount int64 //current value counts
}

type mapEntry struct {
	prefix   string
	fn       LoadFunc
	bfn      BatchLoadFunc
	slotList map[int64]*list.List
}

func newSimpleMapList() *simpleMapList {
	return &simpleMapList{
		ids:     make(map[string]struct{}),
		mapList: make(map[string]*mapEntry),
	}
}

func (sm *simpleMapList) Add(slot int64, value Indicator) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	//add key count
	sm.valueCount++
	//add ids
	sm.ids[value.Id()] = struct{}{}
	//add map list
	if me, ok := sm.mapList[value.Prefix()]; ok {
		if me.fn == nil {
			me.fn = value.LoadFunc()
		}
		if me.bfn == nil {
			me.bfn = value.BatchLoadFunc()
		}
		if _, ok1 := me.slotList[slot]; !ok1 {
			me.slotList[slot] = list.New()
		}
		me.slotList[slot].PushBack(value)
	} else {
		item := &mapEntry{
			prefix:   value.Prefix(),
			fn:       value.LoadFunc(),
			bfn:      value.BatchLoadFunc(),
			slotList: make(map[int64]*list.List),
		}
		newList := list.New()
		newList.PushBack(value)
		item.slotList[slot] = newList
		sm.mapList[value.Prefix()] = item
	}
	return nil
}

type slotValues struct {
	fn         LoadFunc
	bfn        BatchLoadFunc
	indicators []Indicator
}

func (sm *simpleMapList) Get(slot int64) (map[string]*slotValues, error) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	mvs := make(map[string]*slotValues)
	for prefix, me := range sm.mapList {
		if item, ok := me.slotList[slot]; ok {
			indicators := make([]Indicator, 0)
			current := item.Front()
			for current != nil {
				i := current.Value.(Indicator)
				indicators = append(indicators, i)
				current = current.Next()
			}
			mvs[prefix] = &slotValues{
				fn:         me.fn,
				bfn:        me.bfn,
				indicators: indicators,
			}
		}
	}
	return mvs, nil
}

func (sm *simpleMapList) Delete(index int64) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	for _, me := range sm.mapList {
		if item, ok := me.slotList[index]; ok {
			current := item.Front()
			for current != nil {
				i := current.Value.(Indicator)
				delete(sm.ids, i.Id())
				sm.valueCount--
				current = current.Next()
			}
			delete(me.slotList, index)
		}
	}
	return nil
}

func (sm *simpleMapList) Exist(id string) bool {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	_, ok := sm.ids[id]
	return ok
}

func (sm *simpleMapList) GetValueCount() int64 {
	return sm.valueCount
}
