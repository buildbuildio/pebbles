package playground

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultPlayground(t *testing.T) {
	var df DefaultPlayground

	r := httptest.NewRecorder()

	df.ServePlayground(r, nil)

	buf := r.Body

	assert.NotNil(t, buf.String())
}
