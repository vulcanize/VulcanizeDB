// VulcanizeDB
// Copyright © 2019 Vulcanize

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.

// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package cmd

import (
	"errors"
	"fmt"
	"plugin"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/makerdao/vulcanizedb/libraries/shared/factories/event"
	"github.com/makerdao/vulcanizedb/libraries/shared/factories/storage"
	"github.com/makerdao/vulcanizedb/libraries/shared/transformer"
	"github.com/makerdao/vulcanizedb/pkg/config"
	"github.com/makerdao/vulcanizedb/pkg/eth"
	"github.com/makerdao/vulcanizedb/pkg/eth/client"
	"github.com/makerdao/vulcanizedb/pkg/eth/converters"
	"github.com/makerdao/vulcanizedb/pkg/eth/node"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	LogWithCommand           logrus.Entry
	SubCommand               string
	cfgFile                  string
	databaseConfig           config.Database
	diffBlockFromHeadOfChain int64
	genConfig                config.Plugin
	ipc                      string
	maxUnexpectedErrors      int
	recheckHeadersArg        bool
	retryInterval            time.Duration
	startingBlockNumber      int64
	storageDiffsPath         string
	storageDiffsSource       string
)

const (
	pollingInterval  = 7 * time.Second
	validationWindow = 15
)

var rootCmd = &cobra.Command{
	Use:              "vulcanizedb",
	PersistentPreRun: initFuncs,
}

func Execute() {
	logrus.Info("----- Starting vDB -----")
	if err := rootCmd.Execute(); err != nil {
		logrus.Fatal(err)
	}
}

func initFuncs(cmd *cobra.Command, args []string) {
	setViperConfigs()
	logLvlErr := logLevel()
	if logLvlErr != nil {
		logrus.Fatalf("Could not set log level: %s", logLvlErr.Error())
	}

}

func setViperConfigs() {
	ipc = viper.GetString("client.ipcpath")
	storageDiffsPath = viper.GetString("filesystem.storageDiffsPath")
	storageDiffsSource = viper.GetString("storageDiffs.source")
	databaseConfig = config.Database{
		Name:     viper.GetString("database.name"),
		Hostname: viper.GetString("database.hostname"),
		Port:     viper.GetInt("database.port"),
		User:     viper.GetString("database.user"),
		Password: viper.GetString("database.password"),
	}
	viper.Set("database.config", databaseConfig)
}

func logLevel() error {
	lvl, err := logrus.ParseLevel(viper.GetString("log.level"))
	if err != nil {
		return err
	}
	logrus.SetLevel(lvl)
	if lvl > logrus.InfoLevel {
		logrus.SetReportCaller(true)
	}
	logrus.Info("Log level set to ", lvl.String())
	return nil
}

func init() {
	// When searching for env variables, replace dots in config keys with underscores
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file location")
	rootCmd.PersistentFlags().String("database-name", "vulcanize_public", "database name")
	rootCmd.PersistentFlags().Int("database-port", 5432, "database port")
	rootCmd.PersistentFlags().String("database-hostname", "localhost", "database hostname")
	rootCmd.PersistentFlags().String("database-user", "", "database user")
	rootCmd.PersistentFlags().String("database-password", "", "database password")
	rootCmd.PersistentFlags().String("client-ipcPath", "", "location of geth.ipc file")
	rootCmd.PersistentFlags().String("filesystem-storageDiffsPath", "", "location of storage diffs csv file")
	rootCmd.PersistentFlags().String("storageDiffs-source", "csv", "where to get the state diffs: csv or geth")
	rootCmd.PersistentFlags().String("exporter-name", "exporter", "name of exporter plugin")
	rootCmd.PersistentFlags().String("log-level", logrus.InfoLevel.String(), "Log level (trace, debug, info, warn, error, fatal, panic")

	viper.BindPFlag("database.name", rootCmd.PersistentFlags().Lookup("database-name"))
	viper.BindPFlag("database.port", rootCmd.PersistentFlags().Lookup("database-port"))
	viper.BindPFlag("database.hostname", rootCmd.PersistentFlags().Lookup("database-hostname"))
	viper.BindPFlag("database.user", rootCmd.PersistentFlags().Lookup("database-user"))
	viper.BindPFlag("database.password", rootCmd.PersistentFlags().Lookup("database-password"))
	viper.BindPFlag("client.ipcPath", rootCmd.PersistentFlags().Lookup("client-ipcPath"))
	viper.BindPFlag("filesystem.storageDiffsPath", rootCmd.PersistentFlags().Lookup("filesystem-storageDiffsPath"))
	viper.BindPFlag("storageDiffs.source", rootCmd.PersistentFlags().Lookup("storageDiffs-source"))
	viper.BindPFlag("exporter.fileName", rootCmd.PersistentFlags().Lookup("exporter-name"))
	viper.BindPFlag("log.level", rootCmd.PersistentFlags().Lookup("log-level"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
		if err := viper.ReadInConfig(); err == nil {
			logrus.Infof("Using config file: %s\n\n", viper.ConfigFileUsed())
		} else {
			invalidConfigError := "couldn't read config file"
			logrus.Fatalf("%s: %s", invalidConfigError, err.Error())
		}
	} else {
		logrus.Warn("No config file passed with --config flag; attempting to use env vars")
	}
}

func getBlockChain() *eth.BlockChain {
	rpcClient, ethClient := getClients()
	vdbEthClient := client.NewEthClient(ethClient)
	vdbNode := node.MakeNode(rpcClient)
	transactionConverter := converters.NewTransactionConverter(ethClient)
	return eth.NewBlockChain(vdbEthClient, rpcClient, vdbNode, transactionConverter)
}

func getClients() (client.RpcClient, *ethclient.Client) {
	rawRpcClient, err := rpc.Dial(ipc)

	if err != nil {
		LogWithCommand.Fatal(err)
	}
	rpcClient := client.NewRpcClient(rawRpcClient, ipc)
	ethClient := ethclient.NewClient(rawRpcClient)

	return rpcClient, ethClient
}

func prepConfig() error {
	LogWithCommand.Info("configuring plugin")
	names := viper.GetStringSlice("exporter.transformerNames")
	transformers := make(map[string]config.Transformer)
	for _, name := range names {
		transformer := viper.GetStringMapString("exporter." + name)
		p, pOK := transformer["path"]
		if !pOK || p == "" {
			return fmt.Errorf("transformer config is missing `path` value: %s", name)
		}
		r, rOK := transformer["repository"]
		if !rOK || r == "" {
			return fmt.Errorf("transformer config is missing `repository` value: %s", name)
		}
		m, mOK := transformer["migrations"]
		if !mOK || m == "" {
			return fmt.Errorf("transformer config is missing `migrations` value: %s", name)
		}
		mr, mrOK := transformer["rank"]
		if !mrOK || mr == "" {
			return fmt.Errorf("transformer config is missing `rank` value: %s", name)
		}
		rank, err := strconv.ParseUint(mr, 10, 64)
		if err != nil {
			return fmt.Errorf("migration `rank` can't be converted to an unsigned integer: %s", name)
		}
		t, tOK := transformer["type"]
		if !tOK {
			return fmt.Errorf("transformer config is missing `type` value: %s", name)
		}
		transformerType := config.GetTransformerType(t)
		if transformerType == config.UnknownTransformerType {
			return errors.New(`unknown transformer type in exporter config accepted types are "eth_event", "eth_storage"`)
		}

		transformers[name] = config.Transformer{
			Path:           p,
			Type:           transformerType,
			RepositoryPath: r,
			MigrationPath:  m,
			MigrationRank:  rank,
		}
	}

	genConfig = config.Plugin{
		Transformers: transformers,
		FilePath:     "$GOPATH/src/github.com/makerdao/vulcanizedb/plugins",
		Schema:       viper.GetString("exporter.schema"),
		FileName:     viper.GetString("exporter.name"),
		Save:         viper.GetBool("exporter.save"),
		Home:         viper.GetString("exporter.home"),
	}
	return nil
}

func exportTransformers() ([]event.TransformerInitializer, []storage.TransformerInitializer, []transformer.ContractTransformerInitializer, error) {
	// Build plugin generator config
	configErr := prepConfig()
	if configErr != nil {
		return nil, nil, nil, fmt.Errorf("SubCommand %v: failed to to prepare config: %v", SubCommand, configErr)
	}

	// Get the plugin path and load the plugin
	_, pluginPath, pathErr := genConfig.GetPluginPaths()
	if pathErr != nil {
		return nil, nil, nil, fmt.Errorf("SubCommand %v: failed to get plugin paths: %v", SubCommand, pathErr)
	}

	LogWithCommand.Info("linking plugin ", pluginPath)
	plug, openErr := plugin.Open(pluginPath)
	if openErr != nil {
		return nil, nil, nil, fmt.Errorf("SubCommand %v: linking plugin failed: %v", SubCommand, openErr)
	}

	// Load the `Exporter` symbol from the plugin
	LogWithCommand.Info("loading transformers from plugin")
	symExporter, lookupErr := plug.Lookup("Exporter")
	if lookupErr != nil {
		return nil, nil, nil, fmt.Errorf("SubCommand %v: loading Exporter symbol failed: %v", SubCommand, lookupErr)
	}

	// Assert that the symbol is of type Exporter
	exporter, ok := symExporter.(Exporter)
	if !ok {
		return nil, nil, nil, fmt.Errorf("SubCommand %v: plugged-in symbol not of type Exporter", SubCommand)
	}

	// Use the Exporters export method to load the EventTransformerInitializer, StorageTransformerInitializer, and ContractTransformerInitializer sets
	eventTransformerInitializers, storageTransformerInitializers, contractTransformerInitializers := exporter.Export()

	return eventTransformerInitializers, storageTransformerInitializers, contractTransformerInitializers, nil
}

func validateBlockNumberArg(blockNumber int64, argName string) error {
	if blockNumber == -1 {
		return fmt.Errorf("SubCommand: %v: %s argument is required and no value was given", SubCommand, argName)
	}
	return nil
}
