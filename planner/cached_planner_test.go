package planner

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCachedPlannerSimplePlan(t *testing.T) {
	cp := NewCachedPlanner(time.Hour)
	query := `{ getMovies { id }}`

	actual, _ := mustRunPlanner(t, cp, simpleSchema, query, simpleTum)

	expected := `{
		"RootSteps": [
		  {
			"URL": "0",
			"ParentType": "Query",
			"OperationName": null,
			"SelectionSet": "{ getMovies { id } }",
			"InsertionPoint": null,
			"Then": null
		  }
		],
		"ScrubFields": null
	  }`

	assert.JSONEq(t, expected, actual)

	assert.Len(t, cp.cache, 1)
	assert.Len(t, cp.cacheTimers, 1)

	var beforeTime time.Time
	for _, t := range cp.cacheTimers {
		beforeTime = t
	}

	mustRunPlanner(t, cp, simpleSchema, query, simpleTum)

	// cache do not incremented
	assert.Len(t, cp.cache, 1)
	assert.Len(t, cp.cacheTimers, 1)

	var afterTime time.Time
	for _, t := range cp.cacheTimers {
		afterTime = t
	}
	assert.Equal(t, afterTime, beforeTime)
}

func TestCachedPlannerSimplePlanCacheExpires(t *testing.T) {
	cp := NewCachedPlanner(time.Nanosecond)
	query := `{ getMovies { id }}`

	mustRunPlanner(t, cp, simpleSchema, query, simpleTum)

	var beforeTime time.Time
	for _, t := range cp.cacheTimers {
		beforeTime = t
	}

	time.Sleep(time.Nanosecond * 10)

	mustRunPlanner(t, cp, simpleSchema, query, simpleTum)

	// cache do not incremented
	assert.Len(t, cp.cache, 1)
	assert.Len(t, cp.cacheTimers, 1)

	var afterTime time.Time
	for _, t := range cp.cacheTimers {
		afterTime = t
	}

	assert.NotEqual(t, afterTime, beforeTime)
}
