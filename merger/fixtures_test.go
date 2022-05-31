package merger

import (
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

func isEqualSchemas(t *testing.T, expected, actual string) {
	assert.Equal(
		t,
		loadAndFormatSchema(expected),
		loadAndFormatSchema(actual),
		fmt.Sprintf("%s not equls to expected %s", actual, expected),
	)
}

func mustRunMerger(t *testing.T, m Merger, inputs []string) (string, string) {
	t.Helper()

	var inps []*MergeInput
	for i, input := range inputs {
		if input != "" {
			inps = append(inps, &MergeInput{
				Schema: gqlparser.MustLoadSchema(
					&ast.Source{Name: "schema", Input: input},
				),
				URL: strconv.Itoa(i),
			})
		}
	}

	res, err := m.Merge(inps)
	if err != nil {
		panic(err)
	}

	btm, err := json.Marshal(res.TypeURLMap)
	if err != nil {
		panic(err)
	}

	return formatSchema(res.Schema), string(btm)

}

func loadAndFormatSchema(input string) string {
	return formatSchema(gqlparser.MustLoadSchema(&ast.Source{Name: "schema", Input: input}))
}
