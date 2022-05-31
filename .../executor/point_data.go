package executor

import (
	"strconv"
	"strings"
	"sync"
)

type PointData struct {
	Field string
	Index int
	ID    string
}

type PointDataExtractor interface {
	Extract(point string) (*PointData, error)
}

type CachedPointDataExtractor struct {
	cache map[string]*PointData
	sync.RWMutex
}

func (e *CachedPointDataExtractor) Extract(point string) (*PointData, error) {
	e.RLock()
	if v, ok := e.cache[point]; ok {
		e.RUnlock()
		return v, nil
	}
	e.RUnlock()

	field := point
	index := -1
	id := ""

	// points come in the form <field>:<index>#<id> and each of index or id is optional

	// example
	// getUsers:7#User_8

	if strings.Contains(point, "#") {
		idData := strings.Split(point, "#")
		if len(idData) == 2 {
			id = idData[1]
		}

		// use the index data without the id
		field = idData[0]
	}
	// id eq User_8
	// field eq getUsers:7

	if strings.Contains(field, ":") {
		indexData := strings.Split(field, ":")
		indexValue, err := strconv.ParseInt(indexData[1], 0, 32)
		if err != nil {
			return nil, err
		}

		index = int(indexValue)
		field = indexData[0]
	}

	// field eq getUsers
	// index eq 7

	res := &PointData{
		Field: field,
		Index: index,
		ID:    id,
	}
	e.Lock()
	e.cache[point] = res
	e.Unlock()

	return res, nil
}
