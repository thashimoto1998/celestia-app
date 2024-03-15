package app_test

import (
	"fmt"
	"testing"

	coretypes "github.com/tendermint/tendermint/types"
)

type BlockResult struct {
	appVersion int
	appHash    []byte
	dataHash   []byte
}

// In CI use the docker compose script to start up the RPC servers and then provide them as endpoints to the test below.
// Docker compose script that spins up one RPC server per app version: v1, v2.
// Create a block and send it to v0 and v1.
// Use testapp to call process proposal. Need to reset testapp after each test case.
// Save the output from process proposal.
// The test case has a `want` field that contains the expected output.
func TestVersionCompatability(t *testing.T) {
	type testCase struct {
		name  string
		block coretypes.Block
		want  BlockResult
	}
	testCases := []testCase{
		{
			name:  "a valid block on v1 with a single PFB",
			block: coretypes.Block{},
			want: BlockResult{
				appVersion: 1,
				appHash:    []byte{},
				dataHash:   []byte{},
			},
		},
	}

	rpcEndpoints := []string{
		// server running v1
		"localhost:26657",
		// server running v2
		"localhost:26658",
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for rpcEndpoint := range rpcEndpoints {
				fmt.Printf("rpcEndpoint %v", rpcEndpoint)
			}
		})
	}
}
