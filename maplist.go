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
		if _, ok1 := me.slotList[slot]; !ok1 {
			me.slotList[slot] = list.New()
		}
		me.slotList[slot].PushBack(value)
	} else {
		item := &mapEntry{
			slotList: make(map[int64]*list.List),
		}
		newList := list.New()
		newList.PushBack(value)
		item.slotList[slot] = newList
		sm.mapList[value.Prefix()] = item
	}
	return nil
}

func (sm *simpleMapList) Get(slot int64) (map[string][]Indicator, error) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	sv := make(map[string][]Indicator)
	for prefix, me := range sm.mapList {
		if item, ok := me.slotList[slot]; ok {
			indicators := make([]Indicator, 0)
			current := item.Front()
			for current != nil {
				i := current.Value.(Indicator)
				indicators = append(indicators, i)
				current = current.Next()
			}
			if len(indicators) > 0 {
				sv[prefix] = indicators
			}
		}
	}
	return sv, nil
}

func (sm *simpleMapList) Delete(slot int64) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	for _, me := range sm.mapList {
		if item, ok := me.slotList[slot]; ok {
			current := item.Front()
			for current != nil {
				i := current.Value.(Indicator)
				delete(sm.ids, i.Id())
				sm.valueCount--
				current = current.Next()
			}
			delete(me.slotList, slot)
		}
	}
	return nil
}

func (sm *simpleMapList) Exist(value Indicator) bool {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	_, ok := sm.ids[value.Id()]
	return ok
}

func (sm *simpleMapList) GetCount() int64 {
	return sm.valueCount
}
