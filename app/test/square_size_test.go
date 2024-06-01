package app_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/celestiaorg/celestia-app/app"
	"github.com/celestiaorg/celestia-app/app/encoding"
	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/test/txsim"
	"github.com/celestiaorg/celestia-app/test/util/genesis"
	"github.com/celestiaorg/celestia-app/test/util/testnode"
	blobtypes "github.com/celestiaorg/celestia-app/x/blob/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	v1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	oldgov "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	"github.com/cosmos/cosmos-sdk/x/params/types/proposal"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/rand"
)

func TestSquareSizeIntegrationTest(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping square size integration test in short mode.")
	}
	suite.Run(t, new(SquareSizeIntegrationTest))
}

type SquareSizeIntegrationTest struct {
	suite.Suite

	cctx              testnode.Context
	rpcAddr, grpcAddr string
	ecfg              encoding.Config
}

func (s *SquareSizeIntegrationTest) SetupSuite() {
	t := s.T()
	t.Log("setting up square size integration test")
	s.ecfg = encoding.MakeConfig(app.ModuleEncodingRegisters...)
	cfg := testnode.DefaultConfig().
		WithModifiers(genesis.ImmediateProposals(s.ecfg.Codec))

	cctx, rpcAddr, grpcAddr := testnode.NewNetwork(t, cfg)

	s.cctx = cctx
	s.rpcAddr = rpcAddr
	s.grpcAddr = grpcAddr
	err := s.cctx.WaitForNextBlock()
	require.NoError(t, err)
}

// TestSquareSizeUpperBound sets the app's params to specific sizes, then fills the
// block with spam txs to measure that the desired max is getting hit
// The "_Flaky" suffix indicates that the test may fail non-deterministically especially when executed in CI.
func (s *SquareSizeIntegrationTest) TestSquareSizeUpperBound_Flaky() {
	t := s.T()

	type test struct {
		name                  string
		govMaxSquareSize      int
		maxBytes              int
		expectedMaxSquareSize int
		pfbsPerBlock          int
	}

	tests := []test{
		{
			name:                  "default",
			govMaxSquareSize:      appconsts.DefaultGovMaxSquareSize,
			maxBytes:              appconsts.DefaultMaxBytes,
			expectedMaxSquareSize: appconsts.DefaultGovMaxSquareSize,
			pfbsPerBlock:          20,
		},
		{
			name:                  "max bytes constrains square size",
			govMaxSquareSize:      appconsts.DefaultGovMaxSquareSize,
			maxBytes:              appconsts.DefaultMaxBytes,
			expectedMaxSquareSize: appconsts.DefaultGovMaxSquareSize,
			pfbsPerBlock:          40,
		},
		{
			name:                  "gov square size == hardcoded max",
			govMaxSquareSize:      appconsts.DefaultSquareSizeUpperBound,
			maxBytes:              appconsts.DefaultSquareSizeUpperBound * appconsts.DefaultSquareSizeUpperBound * appconsts.ContinuationSparseShareContentSize,
			expectedMaxSquareSize: appconsts.DefaultSquareSizeUpperBound,
			pfbsPerBlock:          40,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s.setBlockSizeParams(t, tt.govMaxSquareSize, tt.maxBytes)
			start, end := s.fillBlocks(100_000, 10, tt.pfbsPerBlock, 20*time.Second)

			// check that we're not going above the specified size and that we hit the specified size
			actualMaxSize := 0
			for i := start; i < end; i++ {
				block, err := s.cctx.Client.Block(s.cctx.GoContext(), &i)
				require.NoError(t, err)
				require.LessOrEqual(t, block.Block.Data.SquareSize, uint64(tt.govMaxSquareSize))

				if block.Block.Data.SquareSize > uint64(actualMaxSize) {
					actualMaxSize = int(block.Block.Data.SquareSize)
				}
			}

			require.Equal(t, tt.expectedMaxSquareSize, actualMaxSize)
		})
	}
}

// fillBlock runs txsim with blob sequences using the provided
// arguments. The start and end blocks are returned.
func (s *SquareSizeIntegrationTest) fillBlocks(blobSize, blobsPerPFB, pfbsPerBlock int, period time.Duration) (start, end int64) {
	t := s.T()
	seqs := txsim.NewBlobSequence(
		txsim.NewRange(blobSize/2, blobSize),
		txsim.NewRange(blobsPerPFB/2, blobsPerPFB),
	).Clone(pfbsPerBlock)

	ctx, cancel := context.WithTimeout(context.Background(), period)
	defer cancel()

	startBlock, err := s.cctx.Client.Block(s.cctx.GoContext(), nil)
	require.NoError(t, err)

	// disable the logger
	zerolog.SetGlobalLevel(zerolog.Disabled)
	_ = txsim.Run(
		ctx,
		[]string{s.rpcAddr},
		[]string{s.grpcAddr},
		s.cctx.Keyring,
		"",
		rand.Int63(),
		time.Second,
		false,
		seqs...,
	)

	endBlock, err := s.cctx.Client.Block(s.cctx.GoContext(), nil)
	require.NoError(t, err)

	return startBlock.Block.Height, endBlock.Block.Height
}

// setBlockSizeParams will use the validator account to set the square size and
// max bytes parameters. It assumes that the governance params have been set to
// allow for fast acceptance of proposals, and will fail the test if the
// parameters are not set as expected.
func (s *SquareSizeIntegrationTest) setBlockSizeParams(t *testing.T, squareSize, maxBytes int) {
	account := "validator"

	bparams := &abci.BlockParams{
		MaxBytes: int64(maxBytes),
		MaxGas:   -1,
	}

	// create and submit a new param change proposal for both params
	change1 := proposal.NewParamChange(
		blobtypes.ModuleName,
		string(blobtypes.KeyGovMaxSquareSize),
		fmt.Sprintf("\"%d\"", squareSize),
	)
	change2 := proposal.NewParamChange(
		baseapp.Paramspace,
		string(baseapp.ParamStoreKeyBlockParams),
		string(s.cctx.Codec.MustMarshalJSON(bparams)),
	)
	content := proposal.NewParameterChangeProposal(
		"title",
		"description",
		[]proposal.ParamChange{change1, change2},
	)
	addr := getAddress(account, s.cctx.Keyring)

	msg, err := oldgov.NewMsgSubmitProposal(
		content,
		sdk.NewCoins(
			sdk.NewCoin(appconsts.BondDenom, sdk.NewInt(1000000000))),
		addr,
	)
	require.NoError(t, err)

	res, err := testnode.SignAndBroadcastTx(s.ecfg, s.cctx.Context, account, msg)
	require.Equal(t, res.Code, abci.CodeTypeOK, res.RawLog)
	require.NoError(t, err)
	resp, err := s.cctx.WaitForTx(res.TxHash, 10)
	require.NoError(t, err)
	require.Equal(t, abci.CodeTypeOK, resp.TxResult.Code)

	require.NoError(t, s.cctx.WaitForNextBlock())

	// query the proposal to get the id
	gqc := v1.NewQueryClient(s.cctx.GRPCClient)
	gresp, err := gqc.Proposals(s.cctx.GoContext(), &v1.QueryProposalsRequest{ProposalStatus: v1.ProposalStatus_PROPOSAL_STATUS_VOTING_PERIOD})
	require.NoError(t, err)
	require.Len(t, gresp.Proposals, 1)

	// create and submit a new vote
	vote := v1.NewMsgVote(getAddress(account, s.cctx.Keyring), gresp.Proposals[0].Id, v1.VoteOption_VOTE_OPTION_YES, "")
	res, err = testnode.SignAndBroadcastTx(s.ecfg, s.cctx.Context, account, vote)
	require.NoError(t, err)
	resp, err = s.cctx.WaitForTx(res.TxHash, 10)
	require.NoError(t, err)
	require.Equal(t, abci.CodeTypeOK, resp.TxResult.Code)

	// wait for the voting period to complete
	time.Sleep(time.Second * 6)

	// check that the parameters got updated as expected
	bqc := blobtypes.NewQueryClient(s.cctx.GRPCClient)
	presp, err := bqc.Params(s.cctx.GoContext(), &blobtypes.QueryParamsRequest{})
	require.NoError(t, err)
	require.Equal(t, uint64(squareSize), presp.Params.GovMaxSquareSize)
	latestHeight, err := s.cctx.LatestHeight()
	require.NoError(t, err)

	for i := 0; i < 10; i++ {
		cpresp, err := s.cctx.Client.ConsensusParams(s.cctx.GoContext(), &latestHeight)
		require.NoError(t, err)
		if err != nil || cpresp == nil {
			time.Sleep(time.Second)
			continue
		}
		require.Equal(t, int64(maxBytes), cpresp.ConsensusParams.Block.MaxBytes)
		break
	}
}
