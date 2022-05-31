package planner

import (
	"crypto/sha1"
	"sync"
	"time"

	"github.com/buildbuildio/pebbles/format"
)

type hashKey [20]byte

type CachedPlanner struct {
	TTL time.Duration

	executor Planner

	cache       map[hashKey]*QueryPlan
	cacheTimers map[hashKey]time.Time

	sync.RWMutex
}

func NewCachedPlanner(ttl time.Duration) *CachedPlanner {
	var sp SequentialPlanner

	return &CachedPlanner{
		TTL:         ttl,
		cache:       make(map[hashKey]*QueryPlan),
		cacheTimers: make(map[hashKey]time.Time),
		executor:    sp,
	}
}

func (cp *CachedPlanner) WithExecutor(e Planner) {
	cp.executor = e
}

func (cp *CachedPlanner) hash(ctx *PlanningContext) hashKey {
	s := format.FormatSelectionSetWithArgs(ctx.Operation.SelectionSet)
	sha1 := sha1.Sum([]byte(s))
	return sha1
}

func (cp *CachedPlanner) clean() {
	var toDelete []hashKey
	ttlnow := time.Now().UTC()
	cp.RLock()
	for hk, v := range cp.cacheTimers {
		if v.Before(ttlnow) {
			toDelete = append(toDelete, hk)
		}
	}
	cp.RUnlock()

	if len(toDelete) > 0 {
		cp.Lock()
		defer cp.Unlock()
		for _, hk := range toDelete {
			delete(cp.cache, hk)
			delete(cp.cacheTimers, hk)
		}
	}
}

func (cp *CachedPlanner) Plan(ctx *PlanningContext) (*QueryPlan, error) {
	hk := cp.hash(ctx)

	cp.clean()
	cp.RLock()
	if res, ok := cp.cache[hk]; ok {
		cp.RUnlock()
		return res, nil
	}
	cp.RUnlock()

	res, err := cp.executor.Plan(ctx)
	if err != nil {
		return nil, err
	}

	cp.Lock()
	defer cp.Unlock()

	cp.cache[hk] = res
	cp.cacheTimers[hk] = time.Now().UTC().Add(cp.TTL)

	return res, nil
}
