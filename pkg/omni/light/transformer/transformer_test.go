// VulcanizeDB
// Copyright © 2018 Vulcanize

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

package transformer_test

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/vulcanize/vulcanizedb/pkg/core"
	"github.com/vulcanize/vulcanizedb/pkg/datastore/postgres"
	"github.com/vulcanize/vulcanizedb/pkg/datastore/postgres/repositories"
	"github.com/vulcanize/vulcanizedb/pkg/omni/light/transformer"
	"github.com/vulcanize/vulcanizedb/pkg/omni/shared/constants"
	"github.com/vulcanize/vulcanizedb/pkg/omni/shared/helpers/test_helpers"
	"github.com/vulcanize/vulcanizedb/pkg/omni/shared/helpers/test_helpers/mocks"
)

var _ = Describe("Transformer", func() {
	var db *postgres.DB
	var err error
	var blockChain core.BlockChain
	var headerRepository repositories.HeaderRepository
	var headerID int64

	BeforeEach(func() {
		db, blockChain = test_helpers.SetupDBandBC()
		headerRepository = repositories.NewHeaderRepository(db)
	})

	AfterEach(func() {
		test_helpers.TearDown(db)
	})

	Describe("SetEvents", func() {
		It("Sets which events to watch from the given contract address", func() {
			watchedEvents := []string{"Transfer", "Mint"}
			t := transformer.NewTransformer("", blockChain, db)
			t.SetEvents(constants.TusdContractAddress, watchedEvents)
			Expect(t.WatchedEvents[constants.TusdContractAddress]).To(Equal(watchedEvents))
		})
	})

	Describe("SetEventAddrs", func() {
		It("Sets which account addresses to watch events for", func() {
			eventAddrs := []string{"test1", "test2"}
			t := transformer.NewTransformer("", blockChain, db)
			t.SetEventAddrs(constants.TusdContractAddress, eventAddrs)
			Expect(t.EventAddrs[constants.TusdContractAddress]).To(Equal(eventAddrs))
		})
	})

	Describe("SetMethods", func() {
		It("Sets which methods to poll at the given contract address", func() {
			watchedMethods := []string{"balanceOf", "totalSupply"}
			t := transformer.NewTransformer("", blockChain, db)
			t.SetMethods(constants.TusdContractAddress, watchedMethods)
			Expect(t.WantedMethods[constants.TusdContractAddress]).To(Equal(watchedMethods))
		})
	})

	Describe("SetMethodAddrs", func() {
		It("Sets which account addresses to poll methods against", func() {
			methodAddrs := []string{"test1", "test2"}
			t := transformer.NewTransformer("", blockChain, db)
			t.SetMethodAddrs(constants.TusdContractAddress, methodAddrs)
			Expect(t.MethodAddrs[constants.TusdContractAddress]).To(Equal(methodAddrs))
		})
	})

	Describe("SetRange", func() {
		It("Sets the block range that the contract should be watched within", func() {
			rng := [2]int64{1, 100000}
			t := transformer.NewTransformer("", blockChain, db)
			t.SetRange(constants.TusdContractAddress, rng)
			Expect(t.ContractRanges[constants.TusdContractAddress]).To(Equal(rng))
		})
	})

	Describe("Init", func() {
		It("Initializes transformer's contract objects", func() {
			headerRepository.CreateOrUpdateHeader(mocks.MockHeader1)
			headerRepository.CreateOrUpdateHeader(mocks.MockHeader3)
			t := transformer.NewTransformer("", blockChain, db)
			t.SetEvents(constants.TusdContractAddress, []string{"Transfer"})
			err = t.Init()
			Expect(err).ToNot(HaveOccurred())

			c, ok := t.Contracts[constants.TusdContractAddress]
			Expect(ok).To(Equal(true))

			Expect(c.StartingBlock).To(Equal(int64(6194632)))
			Expect(c.LastBlock).To(Equal(int64(6194634)))
			Expect(c.Abi).To(Equal(constants.TusdAbiString))
			Expect(c.Name).To(Equal("TrueUSD"))
			Expect(c.Address).To(Equal(constants.TusdContractAddress))
		})

		It("Fails to initialize if first and most recent block numbers cannot be fetched from vDB headers table", func() {
			t := transformer.NewTransformer("", blockChain, db)
			t.SetEvents(constants.TusdContractAddress, []string{"Transfer"})
			err = t.Init()
			Expect(err).To(HaveOccurred())
		})

		It("Does nothing if watched events are unset", func() {
			headerRepository.CreateOrUpdateHeader(mocks.MockHeader1)
			headerRepository.CreateOrUpdateHeader(mocks.MockHeader3)
			t := transformer.NewTransformer("", blockChain, db)
			err = t.Init()
			Expect(err).ToNot(HaveOccurred())

			_, ok := t.Contracts[constants.TusdContractAddress]
			Expect(ok).To(Equal(false))
		})
	})

	Describe("Execute", func() {
		BeforeEach(func() {
			header1, err := blockChain.GetHeaderByNumber(6791668)
			Expect(err).ToNot(HaveOccurred())
			header2, err := blockChain.GetHeaderByNumber(6791669)
			Expect(err).ToNot(HaveOccurred())
			header3, err := blockChain.GetHeaderByNumber(6791670)
			Expect(err).ToNot(HaveOccurred())
			headerRepository.CreateOrUpdateHeader(header1)
			headerID, err = headerRepository.CreateOrUpdateHeader(header2)
			Expect(err).ToNot(HaveOccurred())
			headerRepository.CreateOrUpdateHeader(header3)
		})

		It("Transforms watched contract data into custom repositories", func() {
			t := transformer.NewTransformer("", blockChain, db)
			t.SetEvents(constants.TusdContractAddress, []string{"Transfer"})
			t.SetMethods(constants.TusdContractAddress, nil)

			err = t.Init()
			Expect(err).ToNot(HaveOccurred())

			err = t.Execute()
			Expect(err).ToNot(HaveOccurred())

			log := test_helpers.LightTransferLog{}
			err = db.QueryRowx(fmt.Sprintf("SELECT * FROM light_%s.transfer_event", constants.TusdContractAddress)).StructScan(&log)

			// We don't know vulcID, so compare individual fields instead of complete structures
			Expect(log.HeaderID).To(Equal(headerID))
			Expect(log.From).To(Equal("0x1062a747393198f70F71ec65A582423Dba7E5Ab3"))
			Expect(log.To).To(Equal("0x2930096dB16b4A44Ecd4084EA4bd26F7EeF1AEf0"))
			Expect(log.Value).To(Equal("9998940000000000000000"))
		})

		It("Keeps track of contract-related addresses while transforming event data", func() {
			t := transformer.NewTransformer("", blockChain, db)
			t.SetEvents(constants.TusdContractAddress, []string{"Transfer"})
			t.SetMethods(constants.TusdContractAddress, nil)
			err = t.Init()
			Expect(err).ToNot(HaveOccurred())

			c, ok := t.Contracts[constants.TusdContractAddress]
			Expect(ok).To(Equal(true))

			err = t.Execute()
			Expect(err).ToNot(HaveOccurred())

			b, ok := c.TknHolderAddrs["0x1062a747393198f70F71ec65A582423Dba7E5Ab3"]
			Expect(ok).To(Equal(true))
			Expect(b).To(Equal(true))

			b, ok = c.TknHolderAddrs["0x2930096dB16b4A44Ecd4084EA4bd26F7EeF1AEf0"]
			Expect(ok).To(Equal(true))
			Expect(b).To(Equal(true))

			_, ok = c.TknHolderAddrs["0x09BbBBE21a5975cAc061D82f7b843b1234567890"]
			Expect(ok).To(Equal(false))

			_, ok = c.TknHolderAddrs["0x"]
			Expect(ok).To(Equal(false))
		})

		It("Polls given methods using generated token holder address", func() {
			t := transformer.NewTransformer("", blockChain, db)
			t.SetEvents(constants.TusdContractAddress, []string{"Transfer"})
			t.SetMethods(constants.TusdContractAddress, []string{"balanceOf"})
			err = t.Init()
			Expect(err).ToNot(HaveOccurred())

			err = t.Execute()
			Expect(err).ToNot(HaveOccurred())

			res := test_helpers.BalanceOf{}

			err = db.QueryRowx(fmt.Sprintf("SELECT * FROM light_%s.balanceof_method WHERE who_ = '0x1062a747393198f70F71ec65A582423Dba7E5Ab3' AND block = '6791669'", constants.TusdContractAddress)).StructScan(&res)
			Expect(err).ToNot(HaveOccurred())
			Expect(res.Balance).To(Equal("55849938025000000000000"))
			Expect(res.TokenName).To(Equal("TrueUSD"))

			err = db.QueryRowx(fmt.Sprintf("SELECT * FROM light_%s.balanceof_method WHERE who_ = '0x09BbBBE21a5975cAc061D82f7b843b1234567890' AND block = '6791669'", constants.TusdContractAddress)).StructScan(&res)
			Expect(err).To(HaveOccurred())
		})

		It("Fails if initialization has not been done", func() {
			t := transformer.NewTransformer("", blockChain, db)
			t.SetEvents(constants.TusdContractAddress, []string{"Transfer"})
			t.SetMethods(constants.TusdContractAddress, nil)

			err = t.Execute()
			Expect(err).To(HaveOccurred())
		})
	})
})
