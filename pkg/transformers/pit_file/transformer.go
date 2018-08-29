package pit_file

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/vulcanize/vulcanizedb/pkg/core"
	"github.com/vulcanize/vulcanizedb/pkg/datastore/postgres"
	"github.com/vulcanize/vulcanizedb/pkg/transformers/shared"
)

type PitFileTransformer struct {
	Config     shared.TransformerConfig
	Converter  Converter
	Fetcher    shared.LogFetcher
	Repository Repository
}

type PitFileTransformerInitializer struct {
	Config shared.TransformerConfig
}

func (initializer PitFileTransformerInitializer) NewPitFileTransformer(db *postgres.DB, blockChain core.BlockChain) shared.Transformer {
	converter := PitFileConverter{}
	fetcher := shared.NewFetcher(blockChain)
	repository := NewPitFileRepository(db)
	return PitFileTransformer{
		Config:     initializer.Config,
		Converter:  converter,
		Fetcher:    fetcher,
		Repository: repository,
	}
}

func (transformer PitFileTransformer) Execute() error {
	missingHeaders, err := transformer.Repository.MissingHeaders(transformer.Config.StartingBlockNumber, transformer.Config.EndingBlockNumber)
	if err != nil {
		return err
	}
	for _, header := range missingHeaders {
		topics := [][]common.Hash{{common.HexToHash(shared.PitFileSignature)}}
		matchingLogs, err := transformer.Fetcher.FetchLogs(PitFileConfig.ContractAddress, topics, header.BlockNumber)
		if err != nil {
			return err
		}
		for _, log := range matchingLogs {
			model, err := transformer.Converter.ToModel(PitFileConfig.ContractAddress, shared.PitABI, log)
			if err != nil {
				return err
			}
			err = transformer.Repository.Create(header.Id, log.TxIndex, model)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
