package merger

import (
	"testing"

	"github.com/buildbuildio/pebbles/common"

	"github.com/stretchr/testify/assert"
)

func TestTypeURLMap(t *testing.T) {
	tm := make(TypeURLMap)

	_, ok := tm.Get("test", "test")
	assert.False(t, ok)

	tm.Set("test", "test", "url")

	v, ok := tm.Get("test", "test")
	assert.True(t, ok)
	assert.Equal(t, "url", v)

	urls := tm.GetURLs()

	assert.Equal(t, []string{"url"}, urls)

	tm.SetTypeIsImplementsNode("test")

	isImplements, ok := tm.GetTypeIsImplementsNode("test")
	assert.True(t, ok)
	assert.True(t, isImplements)
}

func TestTypeURLMapNoID(t *testing.T) {
	tm := make(TypeURLMap)

	tm.Set("test", common.IDFieldName, "url")

	_, ok := tm.Get("test", common.IDFieldName)
	assert.False(t, ok)

	tm.Set(common.IDFieldName, "test", "url")

	v, ok := tm.Get(common.IDFieldName, "test")
	assert.True(t, ok)
	assert.Equal(t, "url", v)
}

func TestTypeURLMapGetForType(t *testing.T) {
	tm := make(TypeURLMap)

	tm.Set("test", "test2", "url2")
	tm.Set("test", "test3", "url1")
	tm.Set("test", "test1", "url1")
	tm.Set("other", "test1", "url3")

	urls, ok := tm.GetForType("test")
	assert.True(t, ok)
	assert.EqualValues(t, []string{"url1", "url2"}, urls)

	_, ok = tm.GetForType("test123")
	assert.False(t, ok)
}
