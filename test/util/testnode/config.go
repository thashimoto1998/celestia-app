package testnode

import (
	"fmt"
	"time"

	"github.com/celestiaorg/celestia-app/cmd/celestia-appd/cmd"
	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	v1 "github.com/celestiaorg/celestia-app/pkg/appconsts/v1"
	"github.com/celestiaorg/celestia-app/test/util/genesis"
	"github.com/cosmos/cosmos-sdk/server"
	srvconfig "github.com/cosmos/cosmos-sdk/server/config"
	srvtypes "github.com/cosmos/cosmos-sdk/server/types"
	tmconfig "github.com/tendermint/tendermint/config"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

const (
	DefaultValidatorAccountName = "validator"
)

// Config is the configuration of a test node.
type Config struct {
	Genesis *genesis.Genesis
	// TmConfig is the Tendermint configuration used for the network.
	TmConfig *tmconfig.Config
	// AppConfig is the application configuration of the test node.
	AppConfig *srvconfig.Config
	// AppOptions are the application options of the test node.
	AppOptions *KVAppOptions
	// AppCreator is used to create the application for the testnode.
	AppCreator srvtypes.AppCreator
	// SupressLogs
	SupressLogs bool
}

func (c *Config) WithGenesis(g *genesis.Genesis) *Config {
	c.Genesis = g
	return c
}

// WithTendermintConfig sets the TmConfig and returns the *Config.
func (c *Config) WithTendermintConfig(conf *tmconfig.Config) *Config {
	c.TmConfig = conf
	return c
}

// WithAppConfig sets the AppConfig and returns the Config.
//
// Warning: This method will also overwrite relevant portions of the app config
// to the app options. See the SetFromAppConfig method for more information on
// which values are overwritten.
func (c *Config) WithAppConfig(conf *srvconfig.Config) *Config {
	c.AppConfig = conf
	c.AppOptions.SetFromAppConfig(conf)
	return c
}

// WithAppOptions sets the AppOptions and returns the Config.
//
// Warning: If the app config is set after this, it could overwrite some values.
// See SetFromAppConfig for more information on which values are overwritten.
func (c *Config) WithAppOptions(opts *KVAppOptions) *Config {
	c.AppOptions = opts
	return c
}

// WithAppCreator sets the AppCreator and returns the Config.
func (c *Config) WithAppCreator(creator srvtypes.AppCreator) *Config {
	c.AppCreator = creator
	return c
}

// WithSupressLogs sets the SupressLogs and returns the Config.
func (c *Config) WithSupressLogs(sl bool) *Config {
	c.SupressLogs = sl
	return c
}

// WithTimeoutCommit sets the CommitTimeout and returns the Config.
func (c *Config) WithTimeoutCommit(d time.Duration) *Config {
	c.TmConfig.Consensus.TimeoutCommit = d
	return c
}

// WithFundedAccounts sets the genesis accounts and returns the Config.
func (c *Config) WithFundedAccounts(accounts ...string) *Config {
	c.Genesis = c.Genesis.WithAccounts(
		genesis.NewAccounts(999999999999999999, accounts...)...,
	)
	return c
}

// WithModifiers sets the genesis options and returns the Config.
func (c *Config) WithModifiers(ops ...genesis.Modifier) *Config {
	c.Genesis = c.Genesis.WithModifiers(ops...)
	return c
}

// WithGenesisTime sets the genesis time and returns the Config.
func (c *Config) WithGenesisTime(t time.Time) *Config {
	c.Genesis = c.Genesis.WithGenesisTime(t)
	return c
}

// WithChainID sets the chain ID and returns the Config.
func (c *Config) WithChainID(id string) *Config {
	c.Genesis = c.Genesis.WithChainID(id)
	return c
}

// WithConsensusParams sets the consensus params and returns the Config.
func (c *Config) WithConsensusParams(params *tmproto.ConsensusParams) *Config {
	c.Genesis = c.Genesis.WithConsensusParams(params)
	return c
}

func DefaultConfig() *Config {
	tmcfg := DefaultTendermintConfig()
	tmcfg.Consensus.TimeoutCommit = 1 * time.Millisecond
	cfg := &Config{}
	return cfg.
		WithGenesis(
			genesis.NewDefaultGenesis().
				WithValidators(genesis.NewDefaultValidator(DefaultValidatorAccountName)).
				WithConsensusParams(DefaultConsensusParams()),
		).
		WithTendermintConfig(DefaultTendermintConfig()).
		WithAppOptions(DefaultAppOptions()).
		WithAppConfig(DefaultAppConfig()).
		WithConsensusParams(DefaultConsensusParams()).
		WithAppCreator(cmd.NewAppServer).
		WithSupressLogs(true)
}

type KVAppOptions struct {
	options map[string]interface{}
}

func NewKVAppOptions() *KVAppOptions {
	return &KVAppOptions{options: make(map[string]interface{})}
}

// Get implements AppOptions
func (ao *KVAppOptions) Get(o string) interface{} {
	return ao.options[o]
}

// Set adds an option to the KVAppOptions
func (ao *KVAppOptions) Set(o string, v interface{}) {
	ao.options[o] = v
}

// SetMany adds an option to the KVAppOptions
func (ao *KVAppOptions) SetMany(o map[string]interface{}) {
	for k, v := range o {
		ao.Set(k, v)
	}
}

func (ao *KVAppOptions) SetFromAppConfig(appCfg *srvconfig.Config) {
	opts := map[string]interface{}{
		server.FlagPruning:                     appCfg.Pruning,
		server.FlagPruningKeepRecent:           appCfg.PruningKeepRecent,
		server.FlagPruningInterval:             appCfg.PruningInterval,
		server.FlagMinGasPrices:                appCfg.MinGasPrices,
		server.FlagMinRetainBlocks:             appCfg.MinRetainBlocks,
		server.FlagIndexEvents:                 appCfg.IndexEvents,
		server.FlagStateSyncSnapshotInterval:   appCfg.StateSync.SnapshotInterval,
		server.FlagStateSyncSnapshotKeepRecent: appCfg.StateSync.SnapshotKeepRecent,
		server.FlagHaltHeight:                  appCfg.HaltHeight,
		server.FlagHaltTime:                    appCfg.HaltTime,
	}
	ao.SetMany(opts)
}

// DefaultAppOptions returns the default application options. The options are
// set using the default app config. If the app config is set after this, it
// will overwrite these values.
func DefaultAppOptions() *KVAppOptions {
	opts := NewKVAppOptions()
	opts.SetFromAppConfig(DefaultAppConfig())
	return opts
}

// Deprecated: use DefaultConsensusParams instead.
func DefaultParams() *tmproto.ConsensusParams {
	return DefaultConsensusParams()
}

func DefaultConsensusParams() *tmproto.ConsensusParams {
	consensusParams := types.DefaultConsensusParams()
	consensusParams.Block.TimeIotaMs = 1
	consensusParams.Block.MaxBytes = appconsts.DefaultMaxBytes
	consensusParams.Version.AppVersion = appconsts.LatestVersion
	return consensusParams
}

func DefaultInitialConsensusParams() *tmproto.ConsensusParams {
	consensusParams := types.DefaultConsensusParams()
	consensusParams.Block.TimeIotaMs = 1
	consensusParams.Block.MaxBytes = appconsts.DefaultMaxBytes
	consensusParams.Version.AppVersion = v1.Version
	return consensusParams
}

func DefaultTendermintConfig() *tmconfig.Config {
	tmCfg := tmconfig.DefaultConfig()
	// TimeoutCommit is the duration the node waits after committing a block
	// before starting the next height. This duration influences the time
	// interval between blocks. A smaller TimeoutCommit value could lead to
	// less time between blocks (i.e. shorter block intervals).
	tmCfg.Consensus.TimeoutCommit = 1 * time.Millisecond

	// set the mempool's MaxTxBytes to allow the testnode to accept a
	// transaction that fills the entire square. Any blob transaction larger
	// than the square size will still fail no matter what.
	tmCfg.Mempool.MaxTxBytes = appconsts.DefaultMaxBytes

	// remove all barriers from the testnode being able to accept very large
	// transactions and respond to very queries with large responses (~200MB was
	// chosen only as an arbitrary large number).
	tmCfg.RPC.MaxBodyBytes = 200_000_000

	// set all the ports to random open ones
	tmCfg.RPC.ListenAddress = fmt.Sprintf("tcp://127.0.0.1:%d", GetFreePort())
	tmCfg.P2P.ListenAddress = fmt.Sprintf("tcp://127.0.0.1:%d", GetFreePort())
	tmCfg.RPC.GRPCListenAddress = fmt.Sprintf("tcp://127.0.0.1:%d", GetFreePort())

	return tmCfg
}
