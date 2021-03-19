package main

import (
	"fmt"
	"os"
	"time"

	"github.com/anyswap/CrossChain-Bridge/cmd/utils"
	"github.com/anyswap/CrossChain-Bridge/log"
	"github.com/anyswap/CrossChain-Bridge/mongodb"
	"github.com/anyswap/CrossChain-Bridge/params"
	rpcserver "github.com/anyswap/CrossChain-Bridge/rpc/server"
	"github.com/anyswap/CrossChain-Bridge/worker"
	"github.com/urfave/cli/v2"
)

var (
	clientIdentifier = "swaprouter"
	// Git SHA1 commit hash of the release (set via linker flags)
	gitCommit = ""
	// The app that holds all commands and flags.
	app = utils.NewApp(clientIdentifier, gitCommit, "the swaprouter command line interface")
)

func initApp() {
	// Initialize the CLI app and start action
	app.Action = swaprouter
	app.HideVersion = true // we have a command to print the version
	app.Copyright = "Copyright 2017-2020 The CrossChain-Bridge Authors"
	app.Commands = []*cli.Command{
		adminCommand,
		configCommand,
		scanswapCommand,
		utils.LicenseCommand,
		utils.VersionCommand,
	}
	app.Flags = []cli.Flag{
		utils.ConfigFileFlag,
		utils.RunServerFlag,
		utils.LogFileFlag,
		utils.LogRotationFlag,
		utils.LogMaxAgeFlag,
		utils.VerbosityFlag,
		utils.JSONFormatFlag,
		utils.ColorFormatFlag,
	}
}

func main() {
	initApp()
	if err := app.Run(os.Args); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

func swaprouter(ctx *cli.Context) error {
	utils.SetLogger(ctx)
	if ctx.NArg() > 0 {
		return fmt.Errorf("invalid command: %q", ctx.Args().Get(0))
	}
	exitCh := make(chan struct{})
	isServer := ctx.Bool(utils.RunServerFlag.Name)

	configFile := utils.GetConfigFilePath(ctx)
	config := params.LoadRouterConfig(configFile, isServer)

	if isServer {
		dbConfig := config.Server.MongoDB
		mongodb.MongoServerInit([]string{dbConfig.DBURL}, dbConfig.DBName, dbConfig.UserName, dbConfig.Password)
		worker.StartRouterSwapWork(true)
		time.Sleep(100 * time.Millisecond)
		rpcserver.StartAPIServer()
	} else {
		worker.StartRouterSwapWork(false)
	}

	<-exitCh
	return nil
}
