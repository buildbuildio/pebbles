package queryer

import (
	"testing"

	"github.com/buildbuildio/pebbles/requests"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
)

func TestMultiOpQueryerExtractFilesEmptyRequest(t *testing.T) {
	actual := extractFiles(nil)
	assert.NotNil(t, actual)
}

func TestMultiOpQueryerExtractFiles(t *testing.T) {
	upload1 := &requests.Upload{File: nil, FileName: "file1"}
	upload2 := &requests.Upload{File: nil, FileName: "file2"}
	upload3 := &requests.Upload{File: nil, FileName: "file3"}
	upload4 := &requests.Upload{File: nil, FileName: "file4"}
	upload5 := &requests.Upload{File: nil, FileName: "file5"}
	upload6 := &requests.Upload{File: nil, FileName: "file6"}
	upload7 := &requests.Upload{File: nil, FileName: "file7"}
	upload8 := &requests.Upload{File: nil, FileName: "file8"}
	upload9 := &requests.Upload{File: nil, FileName: "file9"}
	upload10 := &requests.Upload{File: nil, FileName: "file10"}

	input := &requests.Request{
		Variables: map[string]interface{}{
			"stringParam": "hello world",
			"listParam":   []interface{}{"one", "two"},
			"someFile":    upload1,
			"allFiles": []interface{}{
				upload2,
				upload3,
			},
			"dictFiles": map[string]interface{}{
				"file": upload9,
			},
			"arrayFiles": []interface{}{map[string]interface{}{
				"file": upload10,
			}},
			"input": map[string]interface{}{
				"not-an-upload": true,
				"files": []interface{}{
					upload4,
					upload5,
				},
			},
			"these": map[string]interface{}{
				"are": []interface{}{
					upload6,
					map[string]interface{}{
						"some": map[string]interface{}{
							"deeply": map[string]interface{}{
								"nested": map[string]interface{}{
									"uploads": []interface{}{
										upload7,
										upload8,
									},
								},
							},
						},
					},
				},
			},
			"integerParam": 10,
		},
	}

	actual := extractFiles(input)

	expected := &UploadMap{}
	expected.Add(upload1, "someFile")
	expected.Add(upload2, "allFiles.0")
	expected.Add(upload3, "allFiles.1")
	expected.Add(upload4, "input.files.0")
	expected.Add(upload5, "input.files.1")
	expected.Add(upload6, "these.are.0")
	expected.Add(upload7, "these.are.1.some.deeply.nested.uploads.0")
	expected.Add(upload8, "these.are.1.some.deeply.nested.uploads.1")
	expected.Add(upload9, "dictFiles.file")
	expected.Add(upload10, "arrayFiles.0.file")

	am := actual.Map()
	em := expected.Map()

	var vam []string
	for _, v := range am {
		vam = append(vam, v...)
	}

	var vem []string
	for _, v := range em {
		vem = append(vem, v...)
	}

	v1, v2 := lo.Difference(vam, vem)

	assert.Len(t, v1, 0)
	assert.Len(t, v2, 0)

	assert.Equal(t, "hello world", input.Variables["stringParam"])
	assert.Equal(t, "hello world", input.Variables["stringParam"])
	assert.Equal(t, 10, input.Variables["integerParam"])
}
