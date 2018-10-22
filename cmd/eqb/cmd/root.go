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
	"math/rand"
	"os"
	"os/signal"
	"syscall"

	"github.com/kowala-tech/equilibrium/log"
	"github.com/kowala-tech/equilibrium/node"
	"github.com/kowala-tech/equilibrium/p2p"
	"github.com/kowala-tech/equilibrium/params"
	crypto "github.com/libp2p/go-libp2p-crypto"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
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

	isDevNetwork, isTestNetwork bool
)

const rootCmdLongDesc = `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "eqb",
	Short: "A brief description of your application",
	Long:  rootCmdLongDesc,
	Args:  cobra.NoArgs,
	RunE:  runEQB,
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

	rootCmd.Flags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.eqb.yaml)")
	rootCmd.Flags().BoolVar(&isBootstrappingNode, "bootnode", false, "node provides initial configuration information to newly joining nodes so that they may successfully join the overlay network")
	rootCmd.Flags().StringVar(&verbosity, "verbosity", "info", "sets the logger verbosity level ('debug', 'info', 'warn' ,'error', 'dpanic', 'panic', 'fatal'")
	rootCmd.Flags().StringVar(&dataDir, "datadir", "", "data directory for the databases and keystore")
	rootCmd.Flags().StringVar(&nodeKey, "identity", "", "path of the p2p host key file")
	rootCmd.Flags().IntVarP(&listenPort, "port", "p", 32000, "port for incoming connections")
	rootCmd.Flags().StringVar(&listenIP, "ip", "", "ip used for incoming connections")
	rootCmd.Flags().BoolVar(&isDevNetwork, "dev", false, "pre-configured settings for the dev network")
	rootCmd.Flags().BoolVar(&isDevNetwork, "test", false, "pre-configured settings for the test network")

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

		// Search config in home directory with name ".eqb" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".eqb")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

type eqbConfig struct {
	Node   node.Config
	logger *zap.Logger
}

func runEQB(cmd *cobra.Command, args []string) error {
	node := makeNode()
	startNode(node)
	node.Wait()

	return nil
}

func makeNode() *node.Node {
	cfg := eqbConfig{
		Node: node.DefaultConfig,
	}

	if err := setNodeConfig(&cfg.Node); err != nil {
		log.Fatal("Failed to set the node config", zap.Error(err))
	}

	node, err := node.New(context.Background(), &cfg.Node)
	if err != nil {
		log.Fatal("Failed to create the node", zap.Error(err))
	}

	//RegisterArchiveService(node)
	//RegisterMiningService(node)

	return node
}

func setNodeConfig(cfg *node.Config) error {
	setBootstrappingNodes(&cfg.P2P)
	cfg.P2P.IsBootstrappingNode = isBootstrappingNode
	cfg.P2P.ListenAddr = fmt.Sprintf("/ip4/%v/tcp/%v", listenIP, listenPort)

	if nodeKey == "" {
		privKey, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.New(rand.NewSource(0)))
		if err != nil {
			return err
		}
		cfg.P2P.PrivateKey = &privKey
	}

	return nil
}

// setBootstrappingNodes creates a list of bootstrap nodes from the command line
// flags, reverting to pre-configured ones if none have been specified.
func setBootstrappingNodes(cfg *p2p.Config) {
	urls := params.MainBootstrappingNodes
	switch {
	case len(bootstrappingNodes) > 0:
		urls = bootstrappingNodes
	case isTestNetwork:
		urls = params.TestBootstrappingNodes
	case isDevNetwork:
		urls = params.DevBootstrappingNodes
	}

	cfg.BootstrappingNodes = make([]pstore.PeerInfo, 0, len(urls))
	for _, url := range urls {
		peerInfo, err := p2p.ParseURL(url)
		if err != nil {
			log.Error("Invalid Bootstrap url", zap.String("url", url), zap.Error(err))
			continue
		}
		cfg.BootstrappingNodes = append(cfg.BootstrappingNodes, *peerInfo)
	}
}

func startNode(stack *node.Node) {
	log.Info("Starting Node")
	if err := stack.Start(); err != nil {
		log.Fatal("Error starting protocol stack", zap.Error(err))
	}
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(sigCh)
		<-sigCh
		log.Info("Got interrupt, shutting down...")
		go stack.Stop()
		for i := 10; i > 0; i-- {
			<-sigCh
			if i > 1 {
				log.Info("Already shutting down, interrupt more to panic.", zap.Int("times", i-1))
			}
		}
		//debug.Exit() // ensure trace and CPU profile data is flushed.
		//debug.LoudPanic("boom")
	}()
}

/*
func RegisterArchiveService(stack *node.Node) {
	if err := stack.Register(func(ctx *node.ServiceContext) (node.Service, error) {
		return archive.New(ctx)
	}); err != nil {
		log.Fatal("Failed to register the archive service", zap.Error(err))
	}
}


func RegisterMiningService(stack *node.Node) {
	if err := stack.Register(func(ctx *node.ServiceContext) (node.Service, error) {
		var archiveService *archive.Service
		ctx.Service(&archiveService)
		return mining.New(archiveService)
	}); err != nil {
		log.Fatal("Failed to register the mining service", zap.Error(err))
	}
}
*/
