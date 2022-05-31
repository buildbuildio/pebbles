package common

import (
	"sync"

	"github.com/buildbuildio/pebbles/gqlerrors"
	"github.com/samber/lo"
	"github.com/vektah/gqlparser/v2/ast"
)

func IsEqual[T comparable](a []T, b []T) bool {
	if len(a) != len(b) {
		return false
	}

	for i, v := range a {
		if b[i] != v {
			return false
		}
	}
	return true
}

func AsyncMapReduce[T, P, A any](
	payload []T,
	acc A,
	mapFunc func(field T) (P, error),
	reduceFunc func(acc A, value P) A,
) (A, gqlerrors.ErrorList) {
	var errs gqlerrors.ErrorList
	var wg sync.WaitGroup

	wg.Add(len(payload))

	resChan := make(chan P)
	defer close(resChan)

	errChan := make(chan error)
	defer close(errChan)

	doneChan := make(chan struct{})
	defer close(doneChan)

	for _, value := range payload {
		go func(v T) {
			mapRes, err := mapFunc(v)
			if err != nil {
				errChan <- err
				return
			}
			resChan <- mapRes
		}(value)
	}

	go func() {
		for {
			select {
			case res := <-resChan:
				acc = reduceFunc(acc, res)
				wg.Done()
			case err := <-errChan:
				errs = gqlerrors.ExtendErrorList(errs, err)
				wg.Done()
			case <-doneChan:
				return
			}
		}
	}()

	wg.Wait()

	doneChan <- struct{}{}

	if len(errs) > 0 {
		return acc, errs
	}

	return acc, nil
}

// SelectionSetToFields extracts from selection set all data as fields array
// parent definition can be null if we don't want to check anything specific
// if passed don't add fields which're not represented in parent definition
func SelectionSetToFields(selectionSet ast.SelectionSet, parentDef *ast.Definition) []*ast.Field {
	var result []*ast.Field
	for _, s := range selectionSet {
		switch s := s.(type) {
		case *ast.Field:
			if parentDef != nil && !lo.ContainsBy(parentDef.Fields, func(fd *ast.FieldDefinition) bool {
				return fd.Name == s.Name
			}) {
				continue
			}
			result = append(result, s)
		case *ast.InlineFragment:
			if parentDef != nil && s.TypeCondition != parentDef.Name {
				continue
			}
			result = append(result, SelectionSetToFields(s.SelectionSet, parentDef)...)
		}
	}

	return result
}
