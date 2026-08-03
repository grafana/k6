package ageval

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolRegistryOrderAndOverride(t *testing.T) {
	t.Parallel()
	r := newToolRegistry()
	r.add(&evalTool{name: "a", description: "first a"})
	r.add(&evalTool{name: "b", description: "b"})
	r.add(&evalTool{name: "a", description: "override a"}) // skill overrides base tool

	schemas := r.schemas()
	require.Len(t, schemas, 2, "override should not add a duplicate")
	assert.Equal(t, "a", schemas[0].name, "registration order preserved")
	assert.Equal(t, "b", schemas[1].name)
	assert.Equal(t, "override a", schemas[0].description, "later registration wins")

	got, ok := r.get("a")
	require.True(t, ok)
	assert.Equal(t, "override a", got.description)

	_, ok = r.get("missing")
	assert.False(t, ok)
}
