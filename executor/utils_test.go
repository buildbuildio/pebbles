package executor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCopy2DStringArray(t *testing.T) {
	src := [][]string{{"1", "2"}, {"3"}}
	dest := copy2DStringArray(src)
	// values are equal
	assert.Equal(t, dest, src)
	// but pointers are not
	assert.False(t, &src == &dest)
	for i, el := range src {
		//nolint:gosec,scopelint // want to validate pointers
		assert.False(t, &el == &dest[i])
	}
}

func TestMergeMaps(t *testing.T) {
	type m = map[string]interface{}
	type l = []interface{}
	src := m{
		"a": 10,
		"b": l{
			m{
				"id": 1,
				"c": l{
					m{"id": 1, "a": 1},
					m{"id": 2, "a": 2},
					m{"id": 3, "a": 3},
				},
			},
		},
	}
	dst := m{
		"c": 1,
		"b": l{
			m{
				"id": 1,
				"d":  1,
				"c": l{
					m{"id": 1, "b": 1},
					m{"id": 2, "b": 2},
					m{"id": 3, "b": 3},
					m{"id": 4, "b": 4},
				},
			},
		},
	}

	res := mergeMaps(src, dst)

	assert.Equal(t, m{
		"c": 1,
		"a": 10,
		"b": l{
			m{
				"id": 1,
				"d":  1,
				"c": l{
					m{"id": 1, "a": 1, "b": 1},
					m{"id": 2, "a": 2, "b": 2},
					m{"id": 3, "a": 3, "b": 3},
					m{"id": 4, "b": 4},
				},
			},
		},
	}, res)
}

func TestMergeMapsNoIds(t *testing.T) {
	type m = map[string]interface{}
	type l = []interface{}
	src := m{
		"list": l{m{"a": 1}, m{"a": 2}},
	}
	dst := m{
		"list": l{m{"b": 1}, m{"b": 2}},
	}

	res := mergeMaps(src, dst)

	assert.Equal(t, m{
		"list": l{m{"a": 1, "b": 1}, m{"a": 2, "b": 2}},
	}, res)
}

func TestMergeMapsSomeIds(t *testing.T) {
	type m = map[string]interface{}
	type l = []interface{}
	src := m{
		"list": l{m{"a": 1}, m{"a": 2}},
	}
	dst := m{
		"list": l{m{"id": 1}, m{"id": 2}},
	}

	res := mergeMaps(src, dst)

	assert.Equal(t, m{
		"list": l{m{"id": 1, "a": 1}, m{"id": 2, "a": 2}},
	}, res)
}

func TestMergeMapsDifferentObjects(t *testing.T) {
	type m = map[string]interface{}
	type l = []interface{}
	src := m{
		"list": l{m{"a": 1}, m{"a": 2}, m{"a": 3}},
	}
	dst := m{
		"list": l{"1", "2"},
	}

	res := mergeMaps(src, dst)

	assert.Equal(t, m{
		"list": l{m{"a": 1}, m{"a": 2}, m{"a": 3}, "1", "2"},
	}, res)
}

func TestMergeMapsNested(t *testing.T) {
	type m = map[string]interface{}
	src := m{
		"a": m{"b": 1},
	}
	dst := m{
		"a": m{"c": 2},
	}

	res := mergeMaps(src, dst)

	assert.Equal(t, m{
		"a": m{"b": 1, "c": 2},
	}, res)
}
