// Copyright © 2018 NAME HERE <EMAIL ADDRESS>
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
	"os/signal"
	"syscall"

	"github.com/kowala-tech/kcoin/client/log"
	"github.com/kowala-tech/p2p-poc/core"
	"github.com/kowala-tech/p2p-poc/node"
	"github.com/kowala-tech/p2p-poc/params"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile         string
	dataDir         string
	verbosity       int
	isBootstrapNode bool
	bootstrapNodes  = make([]string, 0)
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "kcoin",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	RunE: runKcoin,
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

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.kcoin.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	rootCmd.Flags().BoolP("oracle", "oracle", false, "Enables the oracle service")
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

type GlobalConfig struct {
	Node node.Config
}

func runKcoin(cmd *cobra.Command, args []string) error {

	return nil
}

func RegisterKowalaOracleService(stack *node.Node) {
	if err := stack.Register(func(ctx *node.ServiceContext) (node.Service, error) {
		var kowalaServ *knode.Kowala
		ctx.Service(&kowalaServ)
 		return oracle.New(kowalaServ)
	}); err != nil {
		Fatalf("Failed to register the Kowala Oracle service. %v", err)
	}
}

func makeConfigNode() (*node.Node, GlobalConfig) {
	cfg := GlobalConfig{
		Node: node.DefaultConfig,
		Core: core.DefaultConfig,
	}

	setNodeConfig(&cfg.Node)

	node := node.New(context.Background(), cfg.Node)



	return node, cfg
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

func setNodeConfig(cfg *node.Config) {
	if isBootnode {
		cfg.P2P.IsBootnode = true
	}

	cfg.P2P.ListenAddr = listenAddr
	cfg.P2P.BootstrapNodes = params.NetworkBootnodes
	cfg.P2P.IDGenerationSeed = seed
	cfg.P2P.BootstrapNodes = bootnodes
}
