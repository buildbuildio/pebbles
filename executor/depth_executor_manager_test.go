package executor

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type testDepthExecutorManager struct {
	suite.Suite
	dem *DepthExecutorManager
}

func (suite *testDepthExecutorManager) SetupTest() {
	suite.dem = &DepthExecutorManager{
		result: make(map[string]interface{}),
		pointDataExtractor: &CachedPointDataExtractor{
			cache: make(map[string]*PointData),
		},
	}
}

func (suite *testDepthExecutorManager) TestMergeObject() {
	// the object to insert
	inserted := map[string]interface{}{"hello": "world"}

	// insert the string deeeeep down
	resp := &DepthExecutorResponse{
		ExecutionResults: []*ExecutionResult{{
			InsertionPoint: []string{"hello:5#1", "message", "body:2"},
			Result:         inserted,
		}},
	}
	err := suite.dem.merge(resp)
	suite.NoError(err)

	source := suite.dem.result

	// there should be a list under the key "hello"
	rootList, ok := source["hello"]
	suite.True(ok, "Did not add root list")

	list, ok := rootList.([]interface{})
	suite.True(ok, "Root list is not a list")

	suite.Len(list, 6, "Root list did not have enough entries")

	entry, ok := list[5].(map[string]interface{})
	suite.True(ok, "6th entry wasn't an object")

	// the object we care about is index 5
	message := entry["message"]
	suite.NotNil(message, "Did not add message to object")

	msgObj, ok := message.(map[string]interface{})
	suite.True(ok, "message is not a list")

	// there should be a list under it called body
	bodiesList, ok := msgObj["body"]
	suite.True(ok, "Did not add body list")

	bodies, ok := bodiesList.([]interface{})
	suite.True(ok, "bodies list is not a list")

	suite.Len(bodies, 3, "bodies list did not have enough entries")

	body, ok := bodies[2].(map[string]interface{})
	suite.True(ok, "Body was not an object")

	// make sure that the value is what we expect
	suite.Equal(inserted, body)
}

func (suite *testDepthExecutorManager) TestMergeList() {
	// the object to insert
	inserted := map[string]interface{}{
		"hello": "world",
	}

	// insert the object deeeeep down
	resp := &DepthExecutorResponse{
		ExecutionResults: []*ExecutionResult{{
			InsertionPoint: []string{"hello", "objects:5"},
			Result:         inserted,
		}},
	}
	err := suite.dem.merge(resp)
	suite.NoError(err)

	source := suite.dem.result

	// there should be an object under the key "hello"
	rootEntry, ok := source["hello"]
	suite.True(ok, "Did not add root entry")

	root, ok := rootEntry.(map[string]interface{})
	suite.True(ok, "root object is not an object")

	rootList, ok := root["objects"]
	suite.True(ok, "did not add objects list")

	list, ok := rootList.([]interface{})
	suite.True(ok, "objects is not a list")
	suite.Len(list, 6, "Root list did not have enough entries")

	// make sure that the value is what we expect
	suite.Equal(inserted, list[5])
}

func TestDepthExecutorManager(t *testing.T) {
	suite.Run(t, new(testDepthExecutorManager))
}
