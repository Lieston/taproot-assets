package loadtest

import (
	"time"

	"github.com/jessevdk/go-flags"
)

const (
	// defaultConfigPath is the default path of the configuration file.
	defaultConfigPath = "loadtest.conf"

	// defaultSuiteTimeout is the default timeout for the entire test suite.
	defaultSuiteTimeout = 120 * time.Minute

	// defaultTestTimeout is the default timeout for each test.
	defaultTestTimeout = 10 * time.Minute
)

// User defines the config options for a user in the network.
type User struct {
	Tapd *TapConfig `group:"tapd"  namespace:"tapd"`
}

// TapConfig are the main parameters needed for identifying and creating a grpc
// client to a tapd subsystem.
type TapConfig struct {
	Name string `long:"name" description:"the name of the tapd instance"`
	Host string `long:"host" description:"the host to connect to"`
	Port int    `long:"port" description:"the port to connect to"`

	TLSPath string `long:"tlspath" description:"Path to tapd's TLS certificate, leave empty if TLS is disabled"`
	MacPath string `long:"macpath" description:"Path to tapd's macaroon file"`
}

// BitcoinConfig defines exported config options for the connection to the
// btcd/bitcoind backend.
type BitcoinConfig struct {
	Host     string `long:"host" description:"bitcoind/btcd instance address"`
	Port     int    `long:"port" description:"bitcoind/btcd instance port"`
	User     string `long:"user" description:"bitcoind/btcd user name"`
	Password string `long:"password" description:"bitcoind/btcd password"`
	TLSPath  string `long:"tlspath" description:"Path to btcd's TLS certificate, if TLS is enabled"`
}

// Config holds the main configuration for the performance testing binary.
type Config struct {
	// TestCases is a comma separated list of test cases that will be
	// executed.
	TestCases []string `long:"test-case" description:"the test case that will be executed"`

	// Alice is the configuration for the main user in the network.
	Alice *User `group:"alice" namespace:"alice" description:"alice related configuration"`

	// Bob is the configuration for the secondary user in the network.
	Bob *User `group:"bob" namespace:"bob" description:"bob related configuration"`

	// Bitcoin is the configuration for the bitcoin backend.
	Bitcoin *BitcoinConfig `group:"bitcoin" namespace:"bitcoin" long:"bitcoin" description:"bitcoin client configuration"`

	// BatchSize is the number of assets to mint in a single batch. This is only
	// relevant for some test cases.
	BatchSize int `long:"batch-size" description:"the number of assets to mint in a single batch"`

	// TestSuiteTimeout is the timeout for the entire test suite.
	TestSuiteTimeout time.Duration `long:"test-suite-timeout" description:"the timeout for the entire test suite"`

	// TestTimeout is the timeout for each test.
	TestTimeout time.Duration `long:"test-timeout" description:"the timeout for each test"`
}

// DefaultConfig returns the default configuration for the performance testing
// binary.
func DefaultConfig() Config {
	return Config{
		TestCases: []string{"mint_batch_stress"},
		Alice: &User{
			Tapd: &TapConfig{
				Name: "alice",
			},
		},
		Bob: &User{
			Tapd: &TapConfig{
				Name: "bob",
			},
		},
		BatchSize:        100,
		TestSuiteTimeout: defaultSuiteTimeout,
		TestTimeout:      defaultTestTimeout,
	}
}

// LoadConfig initializes and parses the config using a config file and command
// line options.
//
// The configuration proceeds as follows:
//  1. Start with a default config with sane settings
//  2. Pre-parse the command line to check for an alternative config file
//  3. Load configuration file overwriting defaults with any specified options
//  4. Parse CLI options and overwrite/add any specified options
func LoadConfig() (*Config, error) {
	// Pre-parse the command line options to pick up an alternative config
	// file.
	preCfg := DefaultConfig()
	if _, err := flags.Parse(&preCfg); err != nil {
		return nil, err
	}

	// Next, load any additional configuration options from the file.
	cfg := preCfg
	fileParser := flags.NewParser(&cfg, flags.Default)

	err := flags.NewIniParser(fileParser).ParseFile(defaultConfigPath)
	if err != nil {
		// If it's a parsing related error, then we'll return
		// immediately, otherwise we can proceed as possibly the config
		// file doesn't exist which is OK.
		if _, ok := err.(*flags.IniError); ok { //nolint:gosimple
			return nil, err
		}
	}

	// Finally, parse the remaining command line options again to ensure
	// they take precedence.
	flagParser := flags.NewParser(&cfg, flags.Default)
	if _, err := flagParser.Parse(); err != nil {
		return nil, err
	}

	// Make sure everything we just loaded makes sense.
	cleanCfg, err := ValidateConfig(cfg)
	if err != nil {
		return nil, err
	}

	return cleanCfg, nil
}

// ValidateConfig validates the given configuration and returns a clean version
// of it with sane defaults.
func ValidateConfig(cfg Config) (*Config, error) {
	// TODO (positiveblue): add validation logic.
	return &cfg, nil
}
