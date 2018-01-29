package fakes

import (
	"sort"

	"math/big"

	"github.com/vulcanize/vulcanizedb/pkg/core"
)

type Blockchain struct {
	logs               map[string][]core.Log
	blocks             map[int64]core.Block
	contractAttributes map[string]map[string]string
	blocksChannel      chan core.Block
	WasToldToStop      bool
	node               core.Node
}

func (blockchain *Blockchain) LastBlock() *big.Int {
	var max int64
	for blockNumber := range blockchain.blocks {
		if blockNumber > max {
			max = blockNumber
		}
	}
	return big.NewInt(max)
}

func (blockchain *Blockchain) GetLogs(contract core.Contract, startingBlock *big.Int, endingBlock *big.Int) ([]core.Log, error) {
	return blockchain.logs[contract.Hash], nil
}

func (blockchain *Blockchain) Node() core.Node {
	return blockchain.node
}

func (blockchain *Blockchain) GetAttribute(contract core.Contract, attributeName string, blockNumber *big.Int) (interface{}, error) {
	var result interface{}
	if blockNumber == nil {
		result = blockchain.contractAttributes[contract.Hash+"-1"][attributeName]
	} else {
		result = blockchain.contractAttributes[contract.Hash+blockNumber.String()][attributeName]
	}
	return result, nil
}

func NewBlockchain() *Blockchain {
	return &Blockchain{
		blocks:             make(map[int64]core.Block),
		logs:               make(map[string][]core.Log),
		contractAttributes: make(map[string]map[string]string),
		node:               core.Node{GenesisBlock: "GENESIS", NetworkId: 1, Id: "x123", ClientName: "Geth"},
	}
}

func NewBlockchainWithBlocks(blocks []core.Block) *Blockchain {
	blockNumberToBlocks := make(map[int64]core.Block)
	for _, block := range blocks {
		blockNumberToBlocks[block.Number] = block
	}
	return &Blockchain{
		blocks: blockNumberToBlocks,
	}
}

func (blockchain *Blockchain) GetBlockByNumber(blockNumber int64) core.Block {
	return blockchain.blocks[blockNumber]
}

func (blockchain *Blockchain) AddBlock(block core.Block) {
	blockchain.blocks[block.Number] = block
	blockchain.blocksChannel <- block
}

func (blockchain *Blockchain) SetContractStateAttribute(contractHash string, blockNumber *big.Int, attributeName string, attributeValue string) {
	var key string
	if blockNumber == nil {
		key = contractHash + "-1"
	} else {
		key = contractHash + blockNumber.String()
	}
	contractStateAttributes := blockchain.contractAttributes[key]
	if contractStateAttributes == nil {
		blockchain.contractAttributes[key] = make(map[string]string)
	}
	blockchain.contractAttributes[key][attributeName] = attributeValue
}

func (blockchain *Blockchain) GetAttributes(contract core.Contract) (core.ContractAttributes, error) {
	var contractAttributes core.ContractAttributes
	attributes, ok := blockchain.contractAttributes[contract.Hash+"-1"]
	if ok {
		for key, _ := range attributes {
			contractAttributes = append(contractAttributes, core.ContractAttribute{Name: key, Type: "string"})
		}
	}
	sort.Sort(contractAttributes)
	return contractAttributes, nil
}
