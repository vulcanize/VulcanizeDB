package integration_test

import (
	"io/ioutil"
	"log"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/vulcanize/vulcanizedb/pkg/config"
	"github.com/vulcanize/vulcanizedb/pkg/geth"
	"github.com/vulcanize/vulcanizedb/pkg/history"
	"github.com/vulcanize/vulcanizedb/pkg/repositories/inmemory"
)

func init() {
	log.SetOutput(ioutil.Discard)
}

var _ = Describe("Reading from the Geth blockchain", func() {

	var blockchain *geth.Blockchain
	var inMemory *inmemory.InMemory

	BeforeEach(func() {
		cfg, _ := config.NewConfig("private")
		blockchain = geth.NewBlockchain(cfg.Client.IPCPath)
		inMemory = inmemory.NewInMemory()
	})

	It("reads two blocks", func(done Done) {
		blocks := &inmemory.BlockRepository{InMemory: inMemory}
		validator := history.NewBlockValidator(blockchain, blocks, 2)
		validator.ValidateBlocks()
		Expect(blocks.BlockCount()).To(Equal(2))
		close(done)
	}, 15)

	It("retrieves the genesis block and first block", func(done Done) {
		genesisBlock := blockchain.GetBlockByNumber(int64(0))
		firstBlock := blockchain.GetBlockByNumber(int64(1))
		lastBlockNumber := blockchain.LastBlock()

		Expect(genesisBlock.Number).To(Equal(int64(0)))
		Expect(firstBlock.Number).To(Equal(int64(1)))
		Expect(lastBlockNumber.Int64()).To(BeNumerically(">", 0))
		close(done)
	}, 15)

	It("retrieves the node info", func(done Done) {
		node := blockchain.Node()
		devNetworkGenesisBlock := "0xe5be92145a301820111f91866566e3e99ee344d155569e4556a39bc71238f3bc"
		devNetworkNodeId := float64(1)

		Expect(node.GenesisBlock).To(Equal(devNetworkGenesisBlock))
		Expect(node.NetworkId).To(Equal(devNetworkNodeId))
		Expect(len(node.Id)).To(Equal(128))
		Expect(node.ClientName).To(ContainSubstring("Geth"))

		close(done)
	}, 15)

})
