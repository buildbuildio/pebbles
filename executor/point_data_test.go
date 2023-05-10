package executor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetPointData(t *testing.T) {
	table := []struct {
		point string
		data  *PointData
	}{
		{"foo:2", &PointData{Field: "foo", Index: 2, ID: ""}},
		{"foo#3", &PointData{Field: "foo", Index: -1, ID: "3"}},
		{"foo:2#3", &PointData{Field: "foo", Index: 2, ID: "3"}},
		{"foo#Thing:1337", &PointData{Field: "foo", Index: -1, ID: "Thing:1337"}},
		{"foo:2#Thing:1337", &PointData{Field: "foo", Index: 2, ID: "Thing:1337"}},
	}

	de := &CachedPointDataExtractor{cache: make(map[string]*PointData)}

	for _, row := range table {
		t.Run(row.point, func(t *testing.T) {
			pointData, err := de.Extract(row.point)
			if err != nil {
				t.Error(err.Error())
				return
			}
			assert.Equal(t, row.data, pointData)
		})
	}
}
