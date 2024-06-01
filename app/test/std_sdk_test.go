package app_test

import (
	"sync"
	"testing"
	"time"

	"github.com/celestiaorg/celestia-app/app"
	"github.com/celestiaorg/celestia-app/app/encoding"
	"github.com/celestiaorg/celestia-app/test/util/testnode"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/testutil/mock"
	sdk "github.com/cosmos/cosmos-sdk/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	disttypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	oldgov "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	"github.com/cosmos/cosmos-sdk/x/params/types/proposal"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	abci "github.com/tendermint/tendermint/abci/types"
	tmrand "github.com/tendermint/tendermint/libs/rand"
)

func TestStandardSDKIntegrationTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping SDK integration test in short mode.")
	}
	suite.Run(t, new(StandardSDKIntegrationTestSuite))
}

type StandardSDKIntegrationTestSuite struct {
	suite.Suite

	accounts []string
	cctx     testnode.Context
	ecfg     encoding.Config

	mut            sync.Mutex
	accountCounter int
}

func (s *StandardSDKIntegrationTestSuite) SetupSuite() {
	t := s.T()
	t.Log("setting up integration test suite")

	accounts := make([]string, 35)
	for i := 0; i < len(accounts); i++ {
		accounts[i] = tmrand.Str(9)
	}

	cfg := testnode.DefaultConfig().WithFundedAccounts(accounts...)
	cctx, _, _ := testnode.NewNetwork(t, cfg)
	s.accounts = accounts
	s.ecfg = encoding.MakeConfig(app.ModuleEncodingRegisters...)
	s.cctx = cctx
}

func (s *StandardSDKIntegrationTestSuite) unusedAccount() string {
	s.mut.Lock()
	acc := s.accounts[s.accountCounter]
	s.accountCounter++
	s.mut.Unlock()
	return acc
}

func (s *StandardSDKIntegrationTestSuite) TestStandardSDK() {
	t := s.T()
	type test struct {
		name         string
		msgFunc      func() (msgs []sdk.Msg, signer string)
		hash         string
		expectedCode uint32
	}
	tests := []test{
		{
			name: "send 1 utia",
			msgFunc: func() (msgs []sdk.Msg, signer string) {
				account1, account2 := s.unusedAccount(), s.unusedAccount()
				msgSend := banktypes.NewMsgSend(
					getAddress(account1, s.cctx.Keyring),
					getAddress(account2, s.cctx.Keyring),
					sdk.NewCoins(sdk.NewCoin(app.BondDenom, sdk.NewInt(1))),
				)
				return []sdk.Msg{msgSend}, account1
			},
			expectedCode: abci.CodeTypeOK,
		},
		{
			name: "send 1,000,000 TIA",
			msgFunc: func() (msg []sdk.Msg, signer string) {
				account1, account2 := s.unusedAccount(), s.unusedAccount()
				msgSend := banktypes.NewMsgSend(
					getAddress(account1, s.cctx.Keyring),
					getAddress(account2, s.cctx.Keyring),
					sdk.NewCoins(sdk.NewCoin(app.BondDenom, sdk.NewInt(1000000000000))),
				)
				return []sdk.Msg{msgSend}, account1
			},
			expectedCode: abci.CodeTypeOK,
		},
		{
			name: "delegate 1 TIA",
			msgFunc: func() (msgs []sdk.Msg, signer string) {
				valopAddr := sdk.ValAddress(getAddress("validator", s.cctx.Keyring))
				account1 := s.unusedAccount()
				account1Addr := getAddress(account1, s.cctx.Keyring)
				msg := stakingtypes.NewMsgDelegate(account1Addr, valopAddr, sdk.NewCoin(app.BondDenom, sdk.NewInt(1000000)))
				return []sdk.Msg{msg}, account1
			},
			expectedCode: abci.CodeTypeOK,
		},
		{
			name: "undelegate 1 TIA",
			msgFunc: func() (msgs []sdk.Msg, signer string) {
				valAccAddr := getAddress("validator", s.cctx.Keyring)
				valopAddr := sdk.ValAddress(valAccAddr)
				msg := stakingtypes.NewMsgUndelegate(valAccAddr, valopAddr, sdk.NewCoin(app.BondDenom, sdk.NewInt(1000000)))
				return []sdk.Msg{msg}, testnode.DefaultValidatorAccountName
			},
			expectedCode: abci.CodeTypeOK,
		},
		{
			name: "create validator",
			msgFunc: func() (msgs []sdk.Msg, signer string) {
				pv := mock.NewPV()
				account := s.unusedAccount()
				valopAccAddr := getAddress(account, s.cctx.Keyring)
				valopAddr := sdk.ValAddress(valopAccAddr)
				msg, err := stakingtypes.NewMsgCreateValidator(
					valopAddr,
					pv.PrivKey.PubKey(),
					sdk.NewCoin(app.BondDenom, sdk.NewInt(1)),
					stakingtypes.NewDescription("taco tuesday", "my keybase", "www.celestia.org", "ping @celestiaorg on twitter", "fake validator"),
					stakingtypes.NewCommissionRates(sdk.NewDecWithPrec(6, 0o2), sdk.NewDecWithPrec(12, 0o2), sdk.NewDecWithPrec(1, 0o2)),
					sdk.NewInt(1),
				)
				require.NoError(t, err)
				return []sdk.Msg{msg}, account
			},
			expectedCode: abci.CodeTypeOK,
		},
		{
			name: "create continuous vesting account with a start time in the future",
			msgFunc: func() (msgs []sdk.Msg, signer string) {
				vestAccName := "vesting"
				_, _, err := s.cctx.Keyring.NewMnemonic(vestAccName, keyring.English, "", "", hd.Secp256k1)
				require.NoError(t, err)
				sendAcc := s.unusedAccount()
				sendingAccAddr := getAddress(sendAcc, s.cctx.Keyring)
				vestAccAddr := getAddress(vestAccName, s.cctx.Keyring)
				msg := vestingtypes.NewMsgCreateVestingAccount(
					sendingAccAddr,
					vestAccAddr,
					sdk.NewCoins(sdk.NewCoin(app.BondDenom, sdk.NewInt(1000000))),
					time.Now().Add(time.Hour).Unix(),
					time.Now().Add(time.Hour*2).Unix(),
					false,
				)
				return []sdk.Msg{msg}, sendAcc
			},
			expectedCode: abci.CodeTypeOK,
		},
		{
			name: "create legacy community spend governance proposal",
			msgFunc: func() (msgs []sdk.Msg, signer string) {
				account := s.unusedAccount()
				// Note: this test depends on at least one coin being present
				// in the community pool. Funds land in the community pool due
				// to inflation so if 1 coin is not present in the community
				// pool, consider expanding the block interval or waiting for
				// more blocks to be produced prior to executing this test case.
				coins := sdk.NewCoins(sdk.NewCoin(app.BondDenom, sdk.NewInt(1)))
				content := disttypes.NewCommunityPoolSpendProposal(
					"title",
					"description",
					getAddress(s.unusedAccount(), s.cctx.Keyring),
					coins,
				)
				addr := getAddress(account, s.cctx.Keyring)
				msg, err := oldgov.NewMsgSubmitProposal(
					content,
					sdk.NewCoins(
						sdk.NewCoin(app.BondDenom, sdk.NewInt(1000000000))),
					addr,
				)
				require.NoError(t, err)
				return []sdk.Msg{msg}, account
			},
			expectedCode: abci.CodeTypeOK,
		},
		{
			name: "create legacy text governance proposal",
			msgFunc: func() (msgs []sdk.Msg, signer string) {
				account := s.unusedAccount()
				content, ok := oldgov.ContentFromProposalType("title", "description", "text")
				require.True(t, ok)
				addr := getAddress(account, s.cctx.Keyring)
				msg, err := oldgov.NewMsgSubmitProposal(
					content,
					sdk.NewCoins(
						sdk.NewCoin(app.BondDenom, sdk.NewInt(1000000000))),
					addr,
				)
				require.NoError(t, err)
				return []sdk.Msg{msg}, account
			},
			// plain text proposals have been removed, so we expect an error. "No
			// handler exists for proposal type"
			expectedCode: govtypes.ErrNoProposalHandlerExists.ABCICode(),
		},
		{
			name: "multiple send sdk.Msgs in one sdk.Tx",
			msgFunc: func() (msgs []sdk.Msg, signer string) {
				account1, account2 := s.unusedAccount(), s.unusedAccount()
				msgSend1 := banktypes.NewMsgSend(
					getAddress(account1, s.cctx.Keyring),
					getAddress(account2, s.cctx.Keyring),
					sdk.NewCoins(sdk.NewCoin(app.BondDenom, sdk.NewInt(1))),
				)
				account3 := s.unusedAccount()
				msgSend2 := banktypes.NewMsgSend(
					getAddress(account1, s.cctx.Keyring),
					getAddress(account3, s.cctx.Keyring),
					sdk.NewCoins(sdk.NewCoin(app.BondDenom, sdk.NewInt(1))),
				)
				return []sdk.Msg{msgSend1, msgSend2}, account1
			},
			expectedCode: abci.CodeTypeOK,
		},
		{
			name: "create param change proposal for a blocked parameter",
			msgFunc: func() (msgs []sdk.Msg, signer string) {
				account := s.unusedAccount()
				change := proposal.NewParamChange(stakingtypes.ModuleName, string(stakingtypes.KeyBondDenom), "stake")
				content := proposal.NewParameterChangeProposal("title", "description", []proposal.ParamChange{change})
				addr := getAddress(account, s.cctx.Keyring)
				msg, err := oldgov.NewMsgSubmitProposal(
					content,
					sdk.NewCoins(
						sdk.NewCoin(app.BondDenom, sdk.NewInt(1000000000))),
					addr,
				)
				require.NoError(t, err)
				return []sdk.Msg{msg}, account
			},
			// this parameter is protected by the paramfilter module, and we
			// should expect an error. Due to how errors are bubbled up, we get
			// this code despite wrapping the expected error,
			// paramfilter.ErrBlockedParameter
			expectedCode: govtypes.ErrNoProposalHandlerExists.ABCICode(),
		},
		{
			name: "create param proposal change for a modifiable parameter",
			msgFunc: func() (msgs []sdk.Msg, signer string) {
				account := s.unusedAccount()
				change := proposal.NewParamChange(stakingtypes.ModuleName, string(stakingtypes.KeyMaxValidators), "1")
				content := proposal.NewParameterChangeProposal("title", "description", []proposal.ParamChange{change})
				addr := getAddress(account, s.cctx.Keyring)
				msg, err := oldgov.NewMsgSubmitProposal(
					content,
					sdk.NewCoins(
						sdk.NewCoin(app.BondDenom, sdk.NewInt(1000000000))),
					addr,
				)
				require.NoError(t, err)
				return []sdk.Msg{msg}, account
			},
			expectedCode: abci.CodeTypeOK,
		},
	}

	// sign and submit the transactions
	for i, tt := range tests {
		msgs, signer := tt.msgFunc()
		res, err := testnode.SignAndBroadcastTx(s.ecfg, s.cctx.Context, signer, msgs...)
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, abci.CodeTypeOK, res.Code, tt.name)
		tests[i].hash = res.TxHash
	}

	require.NoError(s.T(), s.cctx.WaitForNextBlock())

	for _, tt := range tests {
		res, err := testnode.QueryTx(s.cctx.Context, tt.hash, true)
		assert.NoError(t, err)
		assert.Equal(t, tt.expectedCode, res.TxResult.Code, tt.name)
	}
}

func getAddress(account string, kr keyring.Keyring) sdk.AccAddress {
	rec, err := kr.Key(account)
	if err != nil {
		panic(err)
	}
	addr, err := rec.GetAddress()
	if err != nil {
		panic(err)
	}
	return addr
}
