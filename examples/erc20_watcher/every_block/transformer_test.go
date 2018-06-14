// Copyright 2018 Vulcanize
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package every_block_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/vulcanize/vulcanizedb/examples/constants"
	"github.com/vulcanize/vulcanizedb/examples/erc20_watcher"
	"github.com/vulcanize/vulcanizedb/examples/erc20_watcher/every_block"
	"github.com/vulcanize/vulcanizedb/examples/mocks"
	"math/big"
	"math/rand"
	"strconv"
)

//allow for setting configuration OR using a default config?
//handle errors

var testContractConfig = erc20_watcher.ContractConfig{
	Address:    constants.DaiContractAddress,
	Abi:        constants.DaiAbiString,
	FirstBlock: int64(4752008),
	LastBlock:  int64(5750050),
	Name:       "Dai",
}

var config = testContractConfig

var _ = Describe("Everyblock transformer", func() {
	var fetcher mocks.Fetcher
	var repository mocks.TotalSupplyRepository
	var transformer every_block.Transformer
	var blockchain mocks.Blockchain
	var initialSupply = "27647235749155415536952630"
	var initialSupplyPlusOne = "27647235749155415536952631"
	var initialSupplyPlusTwo = "27647235749155415536952632"
	var initialSupplyPlusThree = "27647235749155415536952633"
	var defaultLastBlock = big.Int{}

	BeforeEach(func() {
		blockchain = mocks.Blockchain{}
		blockchain.SetLastBlock(&defaultLastBlock)
		fetcher = mocks.Fetcher{Blockchain: &blockchain}
		fetcher.SetSupply(initialSupply)
		repository = mocks.TotalSupplyRepository{}
		repository.SetMissingBlocks([]int64{config.FirstBlock})
		//setting the mock repository to return the first block as the missing blocks

		transformer = every_block.Transformer{
			Fetcher:    &fetcher,
			Repository: &repository,
		}
		transformer.SetConfiguration(config)
	})

	It("fetches and persists the total supply of a token for a single block", func() {
		err := transformer.Execute()
		Expect(err).NotTo(HaveOccurred())

		Expect(len(fetcher.FetchedBlocks)).To(Equal(1))
		Expect(fetcher.FetchedBlocks).To(ConsistOf(config.FirstBlock))
		Expect(fetcher.Abi).To(Equal(config.Abi))
		Expect(fetcher.ContractAddress).To(Equal(config.Address))

		Expect(repository.StartingBlock).To(Equal(config.FirstBlock))
		Expect(repository.EndingBlock).To(Equal(config.LastBlock))
		Expect(len(repository.TotalSuppliesCreated)).To(Equal(1))
		expectedSupply := big.Int{}
		expectedSupply.SetString(initialSupply, 10)
		expectedSupply.Add(&expectedSupply, big.NewInt(1))

		Expect(repository.TotalSuppliesCreated[0].Value).To(Equal(expectedSupply.String()))
	})

	It("retrieves the total supply for every missing block", func() {
		missingBlocks := []int64{
			config.FirstBlock,
			config.FirstBlock + 1,
			config.FirstBlock + 2,
		}
		repository.SetMissingBlocks(missingBlocks)
		transformer.Execute()

		Expect(len(fetcher.FetchedBlocks)).To(Equal(3))
		Expect(fetcher.FetchedBlocks).To(ConsistOf(config.FirstBlock, config.FirstBlock+1, config.FirstBlock+2))
		Expect(fetcher.Abi).To(Equal(config.Abi))
		Expect(fetcher.ContractAddress).To(Equal(config.Address))

		Expect(len(repository.TotalSuppliesCreated)).To(Equal(3))
		Expect(repository.TotalSuppliesCreated[0].Value).To(Equal(initialSupplyPlusOne))
		Expect(repository.TotalSuppliesCreated[1].Value).To(Equal(initialSupplyPlusTwo))
		Expect(repository.TotalSuppliesCreated[2].Value).To(Equal(initialSupplyPlusThree))
	})

	It("uses the set contract configuration", func() {
		repository.SetMissingBlocks([]int64{testContractConfig.FirstBlock})
		transformer.SetConfiguration(testContractConfig)
		err := transformer.Execute()
		Expect(err).NotTo(HaveOccurred())

		Expect(fetcher.FetchedBlocks).To(ConsistOf(testContractConfig.FirstBlock))
		Expect(fetcher.Abi).To(Equal(testContractConfig.Abi))
		Expect(fetcher.ContractAddress).To(Equal(testContractConfig.Address))

		Expect(repository.StartingBlock).To(Equal(testContractConfig.FirstBlock))
		Expect(repository.EndingBlock).To(Equal(testContractConfig.LastBlock))
		Expect(len(repository.TotalSuppliesCreated)).To(Equal(1))
		expectedTokenSupply := every_block.TokenSupply{
			Value:        initialSupplyPlusOne,
			TokenAddress: testContractConfig.Address,
			BlockNumber:  testContractConfig.FirstBlock,
		}
		Expect(repository.TotalSuppliesCreated[0]).To(Equal(expectedTokenSupply))
	})

	It("uses the most recent block if the Config.LastBlock is -1", func() {
		testContractConfig.LastBlock = -1
		transformer.SetConfiguration(testContractConfig)

		randomBlockNumber := rand.Int63()
		numberToString := strconv.FormatInt(randomBlockNumber, 10)
		mostRecentBlock := big.Int{}
		mostRecentBlock.SetString(numberToString, 10)

		blockchain.SetLastBlock(&mostRecentBlock)

		err := transformer.Execute()
		Expect(err).NotTo(HaveOccurred())

		Expect(repository.EndingBlock).To(Equal(randomBlockNumber))
	})

	It("returns an error if the call to get missing blocks fails", func() {
		failureRepository := mocks.FailureRepository{}
		failureRepository.SetMissingBlocksFail(true)
		transformer = every_block.Transformer{
			Fetcher:    &fetcher,
			Repository: &failureRepository,
		}
		err := transformer.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("TestError"))
		Expect(err.Error()).To(ContainSubstring("fetching missing blocks"))
	})

	It("returns an error if the call to the blockchain fails", func() {
		failureBlockchain := mocks.FailureBlockchain{}
		failureBlockchain.SetLastBlock(&defaultLastBlock)
		fetcher := every_block.NewFetcher(failureBlockchain)
		transformer = every_block.Transformer{
			Fetcher:    &fetcher,
			Repository: &repository,
		}
		err := transformer.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("TestError"))
		Expect(err.Error()).To(ContainSubstring("supply"))
	})

	It("returns an error if the call to save the token_supply fails", func() {
		failureRepository := mocks.FailureRepository{}
		failureRepository.SetMissingBlocks([]int64{config.FirstBlock})
		failureRepository.SetCreateFail(true)

		transformer = every_block.Transformer{
			Fetcher:    &fetcher,
			Repository: &failureRepository,
		}
		err := transformer.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("TestError"))
		Expect(err.Error()).To(ContainSubstring("supply"))
	})
})
