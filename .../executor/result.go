package executor

import (
	"errors"
	"fmt"

	"github.com/buildbuildio/pebbles/common"

	"github.com/vektah/gqlparser/v2/ast"
)

// ExtractValueModifyingSource gets object from source by path. If some fields in source are missing, it modifies source to fit the structure.
// for usage information check tests
func ExtractValueModifyingSource(
	extractor PointDataExtractor,
	source map[string]interface{},
	path []string,
) (map[string]interface{}, error) {
	// a pointer to the objects we are modifying
	recent := source

	for _, point := range path {
		// if the point designates an element in the list
		if isListElement(point) {
			pointData, err := extractor.Extract(point)
			if err != nil {
				return nil, err
			}

			// if the field does not exist
			if _, exists := recent[pointData.Field]; !exists {
				recent[pointData.Field] = []interface{}{}
			}

			// it should be a list
			field := recent[pointData.Field]

			targetList, ok := field.([]interface{})
			if !ok {
				return nil, fmt.Errorf("did not encounter a list when expected. Point: %v. Field: %v. Result %v", point, pointData.Field, field)
			}

			// if the field exists but does not have enough spots
			if len(targetList) <= pointData.Index {
				for i := len(targetList) - 1; i < pointData.Index; i++ {
					targetList = append(targetList, map[string]interface{}{})
				}

				// update the list with what we just made
				recent[pointData.Field] = targetList
			}

			// focus on the right element
			recentObj, ok := targetList[pointData.Index].(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("did not encounter a map when expected. Point: %v. Result %v", point, targetList[pointData.Index])
			}
			recent = recentObj
		} else {
			// it's possible that there's an id
			pointData, err := extractor.Extract(point)
			if err != nil {
				return nil, err
			}

			pointField := pointData.Field

			// we are add an object value
			targetObject := recent[pointField]

			// if we haven't created an object there with that field
			if targetObject == nil {
				recent[pointField] = map[string]interface{}{}
			}

			// look there next
			recentObj, ok := recent[pointField].(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("did not encounter a map when expected. Point: %v. Result %v", point, recent[pointField])
			}
			recent = recentObj
		}
	}

	return recent, nil
}

// FindInsertionPoints returns the list of insertion points where provided step should be executed.
// for usage information check tests
//nolint:gocognit,gocyclo // copy paste mostly from original lib
func FindInsertionPoints(
	targetPoints []string,
	selectionSet ast.SelectionSet,
	result map[string]interface{},
	startingPoints [][]string,
) ([][]string, error) {
	oldBranch := copy2DStringArray(startingPoints)

	// track the root of the selection set while  we walk
	selectionSetRoot := selectionSet

	// a place to refer to parts of the results
	resultChunk := result

	// the index to start at
	startingIndex := 0
	if len(oldBranch) > 0 {
		startingIndex = len(oldBranch[0])

		if len(targetPoints) == len(oldBranch[0]) {
			return startingPoints, nil
		}
	}

	// if our starting point is []string{"users:0"} then we know everything so far
	// is along the path of the steps insertion point
	for pointI := startingIndex; pointI < len(targetPoints); pointI++ {
		// the point in the steps insertion path that we want to add
		point := targetPoints[pointI]

		// find the selection node in the AST corresponding to the point
		foundSelection := FindSelection(point, selectionSetRoot)

		// if we didn't find a selection
		if foundSelection == nil {
			return [][]string{}, nil
		}

		// make sure we are looking at the top of the selection set next time
		selectionSetRoot = foundSelection.SelectionSet

		var value = resultChunk

		// the bit of result chunk with the appropriate key should be a list
		rootValue, ok := value[point]
		if !ok {
			return [][]string{}, nil
		}

		// get the type of the object in question
		selectionType := foundSelection.Definition.Type

		if rootValue == nil {
			if selectionType.NonNull {
				return nil, fmt.Errorf("received null for required field: %v", foundSelection.Name)
			}
			return nil, nil
		}

		// if the type is a list
		if selectionType.Elem != nil {
			// make sure the root value is a list
			rootList, ok := rootValue.([]interface{})
			if !ok {
				return nil, fmt.Errorf("root value of result chunk was not a list: %v", rootValue)
			}
			// build up a new list of insertion points
			newInsertionPoints := [][]string{}

			// each value in the result contributes an insertion point
			for entryI, iEntry := range rootList {
				resultEntry, ok := iEntry.(map[string]interface{})
				if !ok {
					return nil, errors.New("entry in result wasn't a map")
				}

				// the point we are going to add to the list
				entryPoint := fmt.Sprintf("%s:%v", foundSelection.Name, entryI)

				newBranchSet := make([][]string, len(oldBranch))
				for i, c := range oldBranch {
					newBranchSet[i] = append(newBranchSet[i], c...)
				}

				// if we are adding to an existing branch
				if len(newBranchSet) > 0 {
					// add the path to the end of this for the entry we just added
					for i, newBranch := range newBranchSet {
						// if we are looking at the last thing in the insertion list
						if pointI == len(targetPoints)-1 {
							// look for an id
							id, err := extractID(resultEntry)
							if err != nil {
								return nil, err
							}

							if id == nil {
								return nil, nil
							}

							// add the id to the entry so that the executor can use it to form its query
							entryPoint = fmt.Sprintf("%s#%v", entryPoint, id)
						}

						// add the point for this entry in the list
						newBranchSet[i] = append(newBranch, entryPoint)
					}
				} else {
					newBranchSet = append(newBranchSet, []string{entryPoint})
				}

				// compute the insertion points for that entry
				entryInsertionPoints, err := FindInsertionPoints(
					targetPoints,
					selectionSetRoot,
					resultEntry,
					newBranchSet,
				)
				if err != nil {
					return nil, err
				}

				// add the list of insertion points to the acumulator
				newInsertionPoints = append(newInsertionPoints, entryInsertionPoints...)
			}

			// return the flat list of insertion points created by our children
			return newInsertionPoints, nil
		}
		// traverse down the resultChunk for the next iteration
		if rootValueMap, ok := rootValue.(map[string]interface{}); ok {
			resultChunk = rootValueMap
		}

		// we are encountering something that isn't a list so it must be an object or a scalar
		// regardless, we just need to add the point to the end of each list
		for i, points := range oldBranch {
			oldBranch[i] = append(points, point)
		}

		if pointI == len(targetPoints)-1 {
			// the root value could be a list in which case the id is the id of the corresponding entry
			// or the root value could be an object in which case the id is the id of the root value

			// if the root value is a list
			if rootList, ok := rootValue.([]interface{}); ok {
				for i := range oldBranch {
					entry, ok := rootList[i].(map[string]interface{})
					if !ok {
						return nil, errors.New("item in root list isn't a map")
					}

					// look up the id of the object
					id, err := extractID(entry)
					if err != nil {
						return nil, err
					}

					if id == nil {
						return nil, nil
					}

					oldBranch[i][pointI] = fmt.Sprintf("%s:%v#%v", oldBranch[i][pointI], i, id)
				}
			} else {
				rootObj, ok := rootValue.(map[string]interface{})
				if !ok {
					return nil, fmt.Errorf("root value of result chunk was not an object. Point: %v Value: %v", point, rootValue)
				}

				for i := range oldBranch {
					// look up the id of the object
					id, err := extractID(rootObj)
					if err != nil {
						return nil, err
					}

					if id == nil {
						return nil, nil
					}

					oldBranch[i][pointI] = fmt.Sprintf("%s#%v", oldBranch[i][pointI], id)
				}
			}
		}
	}

	// return the aggregation
	return oldBranch, nil
}

func extractID(obj map[string]interface{}) (interface{}, error) {
	id, ok := obj[common.IDFieldName]
	if ok {
		return id, nil
	}

	// when requesting union or interface and not querying all possible types
	if _, ok := obj[common.TypenameFieldName]; ok && len(obj) == 1 {
		return nil, nil
	}

	return nil, fmt.Errorf("could not find the id for elements in target list: %v", obj)
}
