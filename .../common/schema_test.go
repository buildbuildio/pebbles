package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsBultinType(t *testing.T) {
	t.Run("true", func(t *testing.T) {
		for _, v := range []string{
			"__typename",
			"__",
			"__other",
		} {
			assert.True(t, IsBuiltinName(v))
		}
	})

	t.Run("false", func(t *testing.T) {
		for _, v := range []string{
			"_typename",
			"_",
			"field",
			"typename",
		} {
			assert.False(t, IsBuiltinName(v))
		}
	})
}

func TestIsRootObjectName(t *testing.T) {
	t.Run("true", func(t *testing.T) {
		for _, v := range []string{
			"Query",
			"Mutation",
			"Subscription",
		} {
			assert.True(t, IsRootObjectName(v))
		}
	})

	t.Run("false", func(t *testing.T) {
		for _, v := range []string{
			"field",
			"query",
			"",
		} {
			assert.False(t, IsBuiltinName(v))
		}
	})
}

func TestIsNodeInterfaceName(t *testing.T) {
	t.Run("true", func(t *testing.T) {
		for _, v := range []string{
			"Node",
		} {
			assert.True(t, IsNodeInterfaceName(v))
		}
	})

	t.Run("false", func(t *testing.T) {
		for _, v := range []string{
			"field",
			"node",
			"",
		} {
			assert.False(t, IsNodeInterfaceName(v))
		}
	})
}
