package requests

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustCheckEqual(t *testing.T, actual *ParseRequestResponse, expected string) {
	b, err := json.Marshal(actual)
	require.NoError(t, err)

	assert.JSONEq(t, expected, string(b))
}
