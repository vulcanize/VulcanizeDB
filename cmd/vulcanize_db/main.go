package main

import (
	"flag"

	"time"

	"os"

	"github.com/vulcanize/vulcanizedb/cmd"
	"github.com/vulcanize/vulcanizedb/pkg/core"
	"github.com/vulcanize/vulcanizedb/pkg/geth"
	"github.com/vulcanize/vulcanizedb/pkg/history"
	"github.com/vulcanize/vulcanizedb/pkg/repositories"
)

const (
	pollingInterval = 7 * time.Second
)

func backFillAllBlocks(blockchain core.Blockchain, repository repositories.Postgres, missingBlocksPopulated chan int) {
	go func() {
		missingBlocksPopulated <- history.PopulateMissingBlocks(blockchain, repository, 0)
	}()
}

func main() {
	ticker := time.NewTicker(pollingInterval)
	defer ticker.Stop()

	environment := flag.String("environment", "", "Environment name")
	flag.Parse()
	config := cmd.LoadConfig(*environment)
	blockchain := geth.NewBlockchain(config.Client.IPCPath)
	repository := cmd.LoadPostgres(config.Database, blockchain.Node())
	validator := history.NewBlockValidator(blockchain, repository, 15)

	missingBlocksPopulated := make(chan int)
	go backFillAllBlocks(blockchain, repository, missingBlocksPopulated)

	for {
		select {
		case <-ticker.C:
			window := validator.ValidateBlocks()
			validator.Log(os.Stdout, window)
		case <-missingBlocksPopulated:
			go backFillAllBlocks(blockchain, repository, missingBlocksPopulated)
		}
	}
}