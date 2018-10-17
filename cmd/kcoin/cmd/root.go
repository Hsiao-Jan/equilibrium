// Copyright © 2018 Kowala SEZC <info@kowala.tech>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/kowala-tech/equilibrium/node"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	// cfgFile represents the config file path.
	cfgFile string

	// verbosity sets the verbosity level.
	verbosity string

	// isBootstrappingNode node performs bootstrapping operations.
	isBootstrappingNode bool

	// dataDir is the data directory for the databases and keystore.
	dataDir string

	// nodePrivKeyFile is the p2p host key file.
	nodeKey string

	// bootstrappingNodes contains the initial list of bootstrapping nodes.
	bootstrappingNodes = make([]string, 0)

	// listenPort represents the port used for incoming connections
	listenPort int 

	// listenIP represents the ip used for incoming connections.
	listenIP string 
)

const rootCmdLongDesc = `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "kcoin",
	Short: "A brief description of your application",
	Long:  rootCmdLongDesc,
	Args:  cobra.NoArgs,
	RunE:  runKcoin,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.Flags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.kcoin.yaml)")
	rootCmd.Flags().BoolVar(&isBootstrappingNode, "bootnode", false, "node provides initial configuration information to newly joining nodes so that they may successfully join the overlay network")
	rootCmd.Flags().StringVar(&verbosity, "verbosity", "info", "sets the logger verbosity level ('debug', 'info', 'warn' ,'error', 'dpanic', 'panic', 'fatal'")
	rootCmd.Flags().StringVar(&dataDir, "datadir", "", "data directory for the databases and keystore")
	rootCmd.Flags().StringVar(&nodeKey, "identity", "", "path of the p2p host key file")
	rootCmd.Flags().IntVarP(&listenPort, "port", "p", 32000, "port for incoming connections")
	rootCmd.Flags().IntVar(&listenIP, "ip", "", "ip used for incoming connections")

}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".kcoin" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".kcoin")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

type KcoinConfig struct {
	Node   node.Config
	logger *zap.Logger
}

func runKcoin(cmd *cobra.Command, args []string) error {

	node := makeNode()

	return nil
}

func makeNode() *node.Node {
	cfg := KcoinConfig{
		Node: node.DefaultConfig,
	}

	setNodeConfig(&cfg.Node)

	node := node.New(context.Background(), cfg.Node)

	return node, cfg
}

func setNodeConfig(cfg *node.Config) {
	cfg.P2P.isBootstrappingNode = isBootstrappingNode
	cfg.P2P.ListenAddr = fmt.Sprintf("/ip4/%v/tcp/%v", listenIP, listenPort)
	cfg.P2P.BootstrappingNodes = bootstrappingNodes
}

func startNode(stack *node.Node) {
	
}

/*
func RegisterKowalaOracleService(stack *node.Node) {
	if err := stack.Register(func(ctx *node.ServiceContext) (node.Service, error) {
		var kowalaServ *knode.Kowala
		ctx.Service(&kowalaServ)
 		return oracle.New(kowalaServ)
	}); err != nil {
		Fatalf("Failed to register the Kowala Oracle service. %v", err)
	}
}




// startNode boots up the system node and all registered protocols, after which
// it unlocks any requested accounts, and starts the RPC/IPC interfaces and the
// validator.
func startNode(stack *node.Node) {
	log.Info("Starting Node")
	if err := stack.Start(); err != nil {
		log.Crit("Error starting protocol stack", "err", err)
	}
	go func() {
		sigc := make(chan os.Signal, 1)
		signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(sigc)
		<-sigc
		log.Info("Got interrupt, shutting down...")
		go stack.Stop()
		for i := 10; i > 0; i-- {
			<-sigc
			if i > 1 {
				log.Warn("Already shutting down, interrupt more to panic.", "times", i-1)
			}
		}
		//debug.Exit() // ensure trace and CPU profile data is flushed.
		//debug.LoudPanic("boom")
	}()
}


