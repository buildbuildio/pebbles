package executor

import (
	"strings"

	"github.com/buildbuildio/pebbles/common"
)

// mergeMaps will merge the right map into the left map recursively
func mergeMaps(left, right map[string]interface{}) map[string]interface{} {
	for key, rightVal := range right {
		if leftVal, present := left[key]; present {
			// If both values is map[string]interface{} - recursively merge it
			lv1, ok1 := leftVal.(map[string]interface{})
			rv1, ok2 := rightVal.(map[string]interface{})

			if ok1 && ok2 {
				left[key] = mergeMaps(lv1, rv1)
				continue
			}

			// If both values []interface{}
			lSlice, ok1 := leftVal.([]interface{})
			rSlice, ok2 := rightVal.([]interface{})

			if ok1 && ok2 {
				lSlice = mergeSlices(lSlice, rSlice)
				left[key] = lSlice
				continue
			}

			left[key] = rightVal
		} else {
			left[key] = rightVal
		}
	}
	return left
}

func getLeftEntityPosition(left []interface{}, id interface{}) int {
	leftEntityPosition := -1
	for lIdx, lv := range left {
		if lMap, ok := lv.(map[string]interface{}); ok {
			if lID, ok := lMap[common.IDFieldName]; ok && lID == id {
				leftEntityPosition = lIdx
				break
			}
		}
	}
	return leftEntityPosition
}

func mergeOrRewriteMap(lSlice []interface{}, rMap map[string]interface{}, id int) []interface{} {
	if v, ok := lSlice[id].(map[string]interface{}); ok {
		lSlice[id] = mergeMaps(v, rMap)
	} else {
		lSlice[id] = rMap
	}
	return lSlice
}

// mergeSlices will merge the right slice into the left slice recursively
func mergeSlices(lSlice, rSlice []interface{}) []interface{} {
	for rIdx, rv := range rSlice {
		// Check if right value from right slice is map[string]interface{}
		if rMap, ok := rv.(map[string]interface{}); ok {
			if rID, ok := rMap[common.IDFieldName]; ok {
				// try to find out an entity with the same id in the left slice
				leftEntityPosition := getLeftEntityPosition(lSlice, rID)

				if leftEntityPosition >= 0 {
					// it's safe to cast as getLeftEntityPosition checks that this element is map
					lSlice[leftEntityPosition] = mergeMaps(lSlice[leftEntityPosition].(map[string]interface{}), rMap)
				} else {
					if rIdx < len(lSlice) {
						lSlice = mergeOrRewriteMap(lSlice, rMap, rIdx)
					} else {
						lSlice = append(lSlice, rv)
					}
				}
			} else {
				// if the map doesn't have the field id we cannot identify the same value
				// add to same position if possible
				if rIdx < len(lSlice) {
					lSlice = mergeOrRewriteMap(lSlice, rMap, rIdx)
					// or append it to the left slice
				} else {
					lSlice = append(lSlice, rv)
				}
			}
		} else {
			lSlice = append(lSlice, rv)
		}
	}

	return lSlice
}

func isListElement(path string) bool {
	if hashLocation := strings.Index(path, "#"); hashLocation > 0 {
		path = path[:hashLocation]
	}
	return strings.Contains(path, ":")
}

// copy2DStringArray deep copiest nested string array
func copy2DStringArray(v [][]string) [][]string {
	res := make([][]string, len(v))
	for i, p := range v {
		res[i] = make([]string, len(p))
		for j, vv := range p {
			res[i][j] = vv
		}
	}
	return res
}
