package healthchecker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type tmpcheck struct {
}

func (t *tmpcheck) Check(endpoint string) bool {
	return true
}

func TestPanic(t *testing.T) {
	checker := &tmpcheck{}

	f, err := NewHealthCheckFilter(checker, 2, 200, []Filter{SimpleMatchFilter("abc")})

	require.NoError(t, err)

	a := "127.0.0.1"
	a2 := "127.0.0.2"
	a3 := "127.0.0.3"
	a4 := "127.0.0.4"

	f.put(a)
	f.put(a2)
	time.Sleep(200 * time.Millisecond)
	f.put(a3)
	time.Sleep(200 * time.Millisecond)
	f.put(a4)

	time.Sleep(time.Second)
}
