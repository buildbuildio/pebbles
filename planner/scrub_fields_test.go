package planner

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScrubFields(t *testing.T) {
	sf1 := make(ScrubFields)
	sf2 := make(ScrubFields)
	tpath := []string{"1", "2"}
	tname := "Type"
	fname1 := "Field1"
	fname2 := "Field2"

	sf1.Set(tpath, tname, fname1)

	actual := sf1.Get(nil, tname)
	assert.Len(t, actual, 0)

	actual = sf1.Get(tpath, tname)
	assert.Equal(t, actual, []string{fname1})

	sf2.Set(tpath, tname, fname2)

	sf1.Merge(nil)

	sf1.Merge(sf2)

	actual = sf1.Get(tpath, tname)
	assert.Equal(t, actual, []string{fname1, fname2})
}

func TestScrubFieldsCleanWithTypename(t *testing.T) {
	sf := make(ScrubFields)

	sf.Set([]string{"a", "b"}, "Test", "id")
	sf.Set([]string{"a", "b"}, "Test", "__typename")

	for _, obj := range []map[string]interface{}{
		{
			"a": []map[string]interface{}{{
				"d": map[string]interface{}{
					"id":         "1",
					"__typename": "Test",
				},
				"b": map[string]interface{}{
					"id":         "1",
					"__typename": "Test",
					"name":       "3",
				},
			}},
			"c": 10,
		},
		{
			"a": []interface{}{map[string]interface{}{
				"d": map[string]interface{}{
					"id":         "1",
					"__typename": "Test",
				},
				"b": map[string]interface{}{
					"id":         "1",
					"__typename": "Test",
					"name":       "3",
				},
			}},
			"c": 10,
		}} {

		sf.Clean(obj)
		expected := `{"c": 10, "a": [{"d": {"id": "1", "__typename": "Test"}, "b": {"name": "3"}}]}`

		b, _ := json.Marshal(obj)

		assert.JSONEq(t, expected, string(b))
	}

}

func TestScrubFieldsCleanWithTypenameDifferentFields(t *testing.T) {
	sf := make(ScrubFields)

	// order here is important for this test case
	sf.Set([]string{"a", "b"}, "Other", "__typename")

	sf.Set([]string{"a", "b"}, "Test", "id")
	sf.Set([]string{"a", "b"}, "Test", "__typename")

	obj := map[string]interface{}{
		"a": []map[string]interface{}{{
			"b": map[string]interface{}{
				"id":         "1",
				"__typename": "Test",
				"name":       "1",
			},
		}, {
			"b": map[string]interface{}{
				"id":         "2",
				"__typename": "Other",
				"name":       "2",
			},
		}},
	}

	sf.Clean(obj)

	expected := `{"a": [{"b": {"name": "1"}}, {"b": {"id": "2", "name": "2"}}]}`

	b, _ := json.Marshal(obj)

	assert.JSONEq(t, expected, string(b))
}

func TestScrubFieldsCleanMissingTypename(t *testing.T) {
	sf := make(ScrubFields)

	sf.Set([]string{"a", "b"}, "Test", "id")
	sf.Set([]string{"a", "b"}, "Test", "__typename")

	obj := map[string]interface{}{
		"a": []map[string]interface{}{{
			"d": map[string]interface{}{
				"id": "1",
			},
			"b": map[string]interface{}{
				"id":   "1",
				"name": "3",
			},
		}},
		"c": 10,
	}

	sf.Clean(obj)

	expected := `{"c": 10, "a": [{"d": {"id": "1"}, "b": {"name": "3"}}]}`

	b, _ := json.Marshal(obj)

	assert.JSONEq(t, expected, string(b))
}

func TestScrubFieldsCleanNotMatchingTypename(t *testing.T) {
	sf := make(ScrubFields)

	sf.Set([]string{"a", "b"}, "Test", "id")
	sf.Set([]string{"a", "b"}, "Test", "__typename")

	obj := map[string]interface{}{
		"a": []map[string]interface{}{{
			"d": map[string]interface{}{
				"id":         "1",
				"__typename": "Test",
			},
			"b": map[string]interface{}{
				"id":         "1",
				"name":       "3",
				"__typename": "OtherTest",
			},
		}},
		"c": 10,
	}

	sf.Clean(obj)

	expected := `{"c": 10, "a": [{"d": {"id": "1", "__typename": "Test"}, "b": {"name": "3", "id": "1", "__typename": "OtherTest"}}]}`

	b, _ := json.Marshal(obj)

	assert.JSONEq(t, expected, string(b))
}

func TestScrubFieldsCleanEmptyObject(t *testing.T) {
	sf := make(ScrubFields)

	sf.Set([]string{"a", "b"}, "Test", "id")
	sf.Set([]string{"a", "b"}, "Test", "__typename")

	obj := map[string]interface{}{}

	sf.Clean(obj)

	expected := `{}`

	b, _ := json.Marshal(obj)

	assert.JSONEq(t, expected, string(b))
}

func TestScrubFieldsCleanEmptySF(t *testing.T) {
	var sf ScrubFields

	obj := map[string]interface{}{}

	sf.Clean(obj)

	expected := `{}`

	b, _ := json.Marshal(obj)

	assert.JSONEq(t, expected, string(b))
}

func TestScrubFieldsRemoveEmptyFields(t *testing.T) {
	sf := make(ScrubFields)

	sf.Set([]string{"a", "b"}, "Test", "id")

	obj := map[string]interface{}{
		"a": []map[string]interface{}{{
			"b": map[string]interface{}{
				"id": "1",
			},
		}},
		"c": 10,
	}

	sf.Clean(obj)

	expected := `{"c": 10}`

	b, _ := json.Marshal(obj)

	assert.JSONEq(t, expected, string(b))
}

func TestScrubFieldsCleanEmptyListResponse(t *testing.T) {
	sf := make(ScrubFields)

	sf.Set([]string{"a"}, "Test", "id")
	sf.Set([]string{"b"}, "Test", "id")

	obj := map[string]interface{}{
		"a": []map[string]interface{}{},
		"b": []interface{}{},
	}

	sf.Clean(obj)

	expected := `{"a": [], "b": []}`

	b, _ := json.Marshal(obj)

	assert.JSONEq(t, expected, string(b))
}

func TestScrubFieldsNullResponse(t *testing.T) {
	sf := make(ScrubFields)

	sf.Set([]string{"a", "c"}, "Test", "id")
	sf.Set([]string{"a"}, "Test", "id")

	obj := map[string]interface{}{
		"a": map[string]interface{}{
			"c":  nil,
			"id": "1",
		},
	}

	sf.Clean(obj)

	expected := `{"a": {"c": null}}`

	b, _ := json.Marshal(obj)

	assert.JSONEq(t, expected, string(b))
}
