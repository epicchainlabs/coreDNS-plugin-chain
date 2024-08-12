package healthchecker

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilter(t *testing.T) {
	f, err := NewRegexpFilter(".*\\.fs\\.neo\\.org")
	require.NoError(t, err)

	require.True(t, f.Match("cdn.fs.neo.org"))
}
