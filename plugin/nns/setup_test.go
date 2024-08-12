package nns

import (
	"context"
	"testing"
	"time"

	"github.com/coredns/caddy"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const testImage = "nspccdev/neofs-aio-testcontainer:0.32.0"

func TestIntegration(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	container := createDockerContainer(ctx, t, testImage)
	defer container.Terminate(ctx)

	c := caddy.NewTestController("dns", "nns http://localhost:30333 -")
	err := setup(c)
	require.NoError(t, err)
	cancel()
}

func TestParseArgs(t *testing.T) {
	for _, tc := range []struct {
		args  string
		valid bool
	}{
		{args: "", valid: false},
		{args: "localhost", valid: false},
		{args: "localhost:30333", valid: false},
		{args: "http://localhost", valid: false},
		{args: "http://localhost:30333 -", valid: true},
		{args: "http://localhost:30333 8b48999931c0607a78e8cb7ed773c572666f2637", valid: true},
		{args: "http://localhost:30333 - domain", valid: true},
		{args: "http://localhost:30333 8b48999931c0607a78e8cb7ed773c572666f2637 domain", valid: true},
		{args: "http://localhost:30333 - domain third", valid: false},
		{args: "http://localhost:30333 8b48999931c0607a78e8cb7ed773c572666f2637 domain third", valid: false},
	} {
		c := caddy.NewTestController("dns", "nns "+tc.args)
		_, err := parseContractParams(c)
		if tc.valid {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
		}
	}
}

func createDockerContainer(ctx context.Context, t *testing.T, image string) testcontainers.Container {
	req := testcontainers.ContainerRequest{
		Image:       image,
		WaitingFor:  wait.NewLogStrategy("aio container started").WithStartupTimeout(30 * time.Second),
		Name:        "coredns-aio",
		Hostname:    "coredns-aio",
		NetworkMode: "host",
	}
	aioC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	return aioC
}
