package app_test

import (
	"testing"
	"time"

	"github.com/celestiaorg/celestia-app/app"
	"github.com/celestiaorg/celestia-app/app/encoding"
	"github.com/celestiaorg/celestia-app/pkg/user"
	"github.com/celestiaorg/celestia-app/test/util/testfactory"
	"github.com/celestiaorg/celestia-app/test/util/testnode"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	abci "github.com/tendermint/tendermint/abci/types"
	tmrand "github.com/tendermint/tendermint/libs/rand"
)

// TestTimeInPrepareProposalContext checks for an edge case where the block time
// needs to be included in the sdk.Context that is being used in the
// antehandlers. If a time is not included in the context, then the second
// transaction in this test will always be filtered out, result in vesting
// accounts never being able to spend funds.
func TestTimeInPrepareProposalContext(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping TestTimeInPrepareProposalContext test in short mode.")
	}
	accounts := make([]string, 35)
	for i := 0; i < len(accounts); i++ {
		accounts[i] = tmrand.Str(9)
	}
	cfg := testnode.DefaultConfig().WithFundedAccounts(accounts...)
	cctx, _, _ := testnode.NewNetwork(t, cfg)
	ecfg := encoding.MakeConfig(app.ModuleEncodingRegisters...)
	vestAccName := "vesting"
	type test struct {
		name         string
		msgFunc      func() (msgs []sdk.Msg, signer string)
		expectedCode uint32
	}
	tests := []test{
		{
			name: "create continuous vesting account with a start time in the future",
			msgFunc: func() (msgs []sdk.Msg, signer string) {
				_, _, err := cctx.Keyring.NewMnemonic(vestAccName, keyring.English, "", "", hd.Secp256k1)
				require.NoError(t, err)
				sendAcc := accounts[0]
				sendingAccAddr := testfactory.GetAddress(cctx.Keyring, sendAcc)
				vestAccAddr := testfactory.GetAddress(cctx.Keyring, vestAccName)
				msg := vestingtypes.NewMsgCreateVestingAccount(
					sendingAccAddr,
					vestAccAddr,
					sdk.NewCoins(sdk.NewCoin(app.BondDenom, sdk.NewInt(1000000))),
					time.Now().Unix(),
					time.Now().Add(time.Second*100).Unix(),
					false,
				)
				return []sdk.Msg{msg}, sendAcc
			},
			expectedCode: abci.CodeTypeOK,
		},
		{
			name: "send funds from the vesting account after it has been created",
			msgFunc: func() (msgs []sdk.Msg, signer string) {
				sendAcc := accounts[1]
				sendingAccAddr := testfactory.GetAddress(cctx.Keyring, sendAcc)
				vestAccAddr := testfactory.GetAddress(cctx.Keyring, vestAccName)
				msg := banktypes.NewMsgSend(
					vestAccAddr,
					sendingAccAddr,
					sdk.NewCoins(sdk.NewCoin(app.BondDenom, sdk.NewInt(1))),
				)
				return []sdk.Msg{msg}, vestAccName
			},
			expectedCode: abci.CodeTypeOK,
		},
	}

	// sign and submit the transactions
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msgs, account := tt.msgFunc()
			addr := testfactory.GetAddress(cctx.Keyring, account)
			signer, err := user.SetupSigner(cctx.GoContext(), cctx.Keyring, cctx.GRPCClient, addr, ecfg)
			require.NoError(t, err)
			res, err := signer.SubmitTx(cctx.GoContext(), msgs, user.SetGasLimit(1000000), user.SetFee(1))
			if tt.expectedCode != abci.CodeTypeOK {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.NotNil(t, res)
			assert.Equal(t, tt.expectedCode, res.Code, res.RawLog)
		})
	}
}
