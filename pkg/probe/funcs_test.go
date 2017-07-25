package probe

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestEnvFunc(t *testing.T) {
	ctx := context.Background()
	r, err := EnvFunc(ctx)
	fmt.Println(r.Data)
	assert.NoError(t, err)
	assert.True(t, len(r.Data) > 0)
}

func TestHostInfoFunc(t *testing.T) {
	ctx := context.Background()
	r, err := HostInfoFunc(ctx)
	fmt.Println(r.Data)
	assert.NoError(t, err)
	assert.True(t, len(r.Data) > 0)
}

func TestProbeFuncs(t *testing.T) {
	//ctx := context.Background()
	//probe.probeFuncs
}
