package app_test

import (
	"fmt"
	"github.com/celestiaorg/celestia-app/app"
	"github.com/celestiaorg/celestia-app/app/encoding"
	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	appns "github.com/celestiaorg/celestia-app/pkg/namespace"
	testutil "github.com/celestiaorg/celestia-app/test/util"
	"github.com/celestiaorg/celestia-app/test/util/blobfactory"
	"github.com/celestiaorg/celestia-app/test/util/testfactory"
	"github.com/cosmos/cosmos-sdk/codec"
	hd "github.com/cosmos/cosmos-sdk/crypto/hd"
	keyring "github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/stretchr/testify/require"
	abci "github.com/tendermint/tendermint/abci/types"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	coretypes "github.com/tendermint/tendermint/types"
	"testing"
)

func TestNonDeterminismBetweenAppVersions(t *testing.T) {
	const (
		numBlobTxs, numNormalTxs = 5, 5
	)

	// Set up testapp with genesis state
	testApp := testutil.NewTestApp()

	enc := encoding.MakeConfig(app.ModuleEncodingRegisters...)
	kr, pubKeys := DeterministicKeyRing(enc.Codec)

	var addresses []string
	recs, _ := kr.List()
	// Get name of record for the accounts
	for _, account := range recs {
		addresses = append(addresses, account.Name)
	}

	// Apply the desired genesis state
	_, _, err := testutil.ApplyGenesisState(testApp, pubKeys, 1_000_000_000, app.DefaultInitialConsensusParams())
	require.NoError(t, err)

	accinfos := queryAccountInfo(testApp, addresses, kr)

	// Create 5 deterministic sdk txs
	normalTxs := testutil.SendTxsWithAccounts(
		t,
		testApp,
		enc.TxConfig,
		kr,
		1000,
		addresses[0],
		addresses[:numNormalTxs],
		testutil.ChainID,
	)

	// Create 5 deterministic blob txs
	blobTxs := blobfactory.ManyMultiBlobTx(t, enc.TxConfig, kr, testutil.ChainID, addresses[numBlobTxs+1:], accinfos[numBlobTxs+1:], testfactory.Repeat([]*tmproto.Blob{
		New(HardcodedNamespace(), []byte{1}, appconsts.DefaultShareVersion),
	}, numBlobTxs))

	// Deliver sdk txs
	for _, tx := range normalTxs {
		resp := testApp.DeliverTx(abci.RequestDeliverTx{Tx: tx})
		require.EqualValues(t, 0, resp.Code, resp.Log)
	}

	// Deliver blob txs
	for _, tx := range blobTxs {
		blobTx, ok := coretypes.UnmarshalBlobTx(tx)
		require.True(t, ok)
		resp := testApp.DeliverTx(abci.RequestDeliverTx{Tx: blobTx.Tx})
		require.EqualValues(t, 0, resp.Code, resp.Log)
	}

	// Commit the state
	testApp.Commit()

	// Get the app hash
	appHash := testApp.LastCommitID().Hash
	fmt.Println("AppHash:", appHash)
}

func HardcodedNamespace() appns.Namespace {
	return appns.Namespace{
		Version: 0,
		ID:      []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 37, 67, 154, 200, 228, 130, 74, 147, 162, 11},
	}
}

func DeterministicKeyRing(cdc codec.Codec) (keyring.Keyring, []types.PubKey) {
	mnemonics := []string{
		"great myself congress genuine scale muscle view uncover pipe miracle sausage broccoli lonely swap table foam brand turtle comic gorilla firm mad grunt hazard",
		"cheap job month trigger flush cactus chest juice dolphin people limit crunch curious secret object beach shield snake hunt group sketch cousin puppy fox",
		"oil suffer bamboo one better attack exist dolphin relief enforce cat asset raccoon lava regret found love certain plunge grocery accuse goat together kiss",
		"giraffe busy subject doll jump drama sea daring again club spend toe mind organ real liar permit refuse change opinion donkey job cricket speed",
		"fee vapor thing fish fan memory negative raven cram win quantum ozone job mirror shoot sting quiz black apart funny sort cancel friend curtain",
		"skin beef review pilot tooth act any alarm there only kick uniform ticket material cereal radar ethics list unlock method coral smooth street frequent",
		"ecology scout core guard load oil school effort near alcohol fancy save cereal owner enforce impact sand husband trophy solve amount fish festival sell",
		"used describe angle twin amateur pyramid bitter pool fluid wing erode rival wife federal curious drink battle put elbow mandate another token reveal tone",
		"reason fork target chimney lift typical fine divorce mixture web robot kiwi traffic stove miss crane welcome camp bless fuel october riot pluck ordinary",
		"undo logic mobile modify master force donor rose crumble forget plate job canal waste turn damp sure point deposit hazard quantum car annual churn",
		"charge subway treat loop donate place loan want grief leg message siren joy road exclude match empty enforce vote meadow enlist vintage wool involve",
	}
	kb := keyring.NewInMemory(cdc)
	pubKeys := make([]types.PubKey, len(mnemonics))
	for idx, mnemonic := range mnemonics {
		rec, err := kb.NewAccount(fmt.Sprintf("account-%d", idx), mnemonic, "", "", hd.Secp256k1)
		if err != nil {
			panic(err)
		}
		pubKey, err := rec.GetPubKey()
		if err != nil {
			panic(err)
		}
		pubKeys[idx] = pubKey
	}
	return kb, pubKeys
}

// New creates a new tmproto.Blob from the provided data
func New(ns appns.Namespace, blob []byte, shareVersion uint8) *tmproto.Blob {
	return &tmproto.Blob{
		NamespaceId:      ns.ID,
		Data:             blob,
		ShareVersion:     uint32(shareVersion),
		NamespaceVersion: uint32(ns.Version),
	}
}
