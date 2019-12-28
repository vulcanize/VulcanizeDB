// VulcanizeDB
// Copyright © 2019 Vulcanize

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

package storage_test

import (
	"math/rand"

	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/vulcanize/vulcanizedb/libraries/shared/factories/storage"
	"github.com/vulcanize/vulcanizedb/libraries/shared/mocks"
	"github.com/vulcanize/vulcanizedb/libraries/shared/storage/utils"
	"github.com/vulcanize/vulcanizedb/pkg/fakes"
)

var _ = Describe("Storage transformer", func() {
	var (
		storageKeysLookup *mocks.MockStorageKeysLookup
		repository        *mocks.MockStorageRepository
		t                 storage.Transformer
	)

	BeforeEach(func() {
		storageKeysLookup = &mocks.MockStorageKeysLookup{}
		repository = &mocks.MockStorageRepository{}
		t = storage.Transformer{
			HashedAddress:     common.Hash{},
			StorageKeysLookup: storageKeysLookup,
			Repository:        repository,
		}
	})

	It("returns the contract address being watched", func() {
		fakeAddress := utils.HexToKeccak256Hash("0x12345")
		t.HashedAddress = fakeAddress

		Expect(t.KeccakContractAddress()).To(Equal(fakeAddress))
	})

	It("looks up metadata for storage key", func() {
		t.Execute(utils.PersistedStorageDiff{})

		Expect(storageKeysLookup.LookupCalled).To(BeTrue())
	})

	It("returns error if lookup fails", func() {
		storageKeysLookup.LookupErr = fakes.FakeError

		err := t.Execute(utils.PersistedStorageDiff{})

		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError(fakes.FakeError))
	})

	It("creates storage row with decoded data", func() {
		fakeMetadata := utils.StorageValueMetadata{Type: utils.Address}
		storageKeysLookup.Metadata = fakeMetadata
		rawValue := common.HexToAddress("0x12345")
		fakeBlockNumber := 123
		fakeBlockHash := "0x67890"
		fakeRow := utils.PersistedStorageDiff{
			ID: rand.Int63(),
			StorageDiffInput: utils.StorageDiffInput{
				HashedAddress: common.Hash{},
				BlockHash:     common.HexToHash(fakeBlockHash),
				BlockHeight:   fakeBlockNumber,
				StorageKey:    common.Hash{},
				StorageValue:  rawValue.Hash(),
			},
		}

		err := t.Execute(fakeRow)

		Expect(err).NotTo(HaveOccurred())
		Expect(repository.PassedDiffID).To(Equal(fakeRow.ID))
		Expect(repository.PassedMetadata).To(Equal(fakeMetadata))
		Expect(repository.PassedValue.(string)).To(Equal(rawValue.Hex()))
	})

	It("returns error if creating row fails", func() {
		rawValue := common.HexToAddress("0x12345")
		fakeMetadata := utils.StorageValueMetadata{Type: utils.Address}
		storageKeysLookup.Metadata = fakeMetadata
		repository.CreateErr = fakes.FakeError
		diff := utils.PersistedStorageDiff{StorageDiffInput: utils.StorageDiffInput{StorageValue: rawValue.Hash()}}

		err := t.Execute(diff)

		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError(fakes.FakeError))
	})

	Describe("when a storage row contains more than one item packed in storage", func() {
		var (
			rawValue        = common.HexToAddress("000000000000000000000000000000000000000000000002a300000000002a30")
			fakeBlockNumber = 123
			fakeBlockHash   = "0x67890"
			packedTypes     = make(map[int]utils.ValueType)
		)
		packedTypes[0] = utils.Uint48
		packedTypes[1] = utils.Uint48

		var fakeMetadata = utils.StorageValueMetadata{
			Name:        "",
			Keys:        nil,
			Type:        utils.PackedSlot,
			PackedTypes: packedTypes,
		}

		It("passes the decoded data items to the repository", func() {
			storageKeysLookup.Metadata = fakeMetadata
			fakeRow := utils.PersistedStorageDiff{
				ID: rand.Int63(),
				StorageDiffInput: utils.StorageDiffInput{
					HashedAddress: common.Hash{},
					BlockHash:     common.HexToHash(fakeBlockHash),
					BlockHeight:   fakeBlockNumber,
					StorageKey:    common.Hash{},
					StorageValue:  rawValue.Hash(),
				},
			}

			err := t.Execute(fakeRow)

			Expect(err).NotTo(HaveOccurred())
			Expect(repository.PassedDiffID).To(Equal(fakeRow.ID))
			Expect(repository.PassedMetadata).To(Equal(fakeMetadata))
			expectedPassedValue := make(map[int]string)
			expectedPassedValue[0] = "10800"
			expectedPassedValue[1] = "172800"
			Expect(repository.PassedValue.(map[int]string)).To(Equal(expectedPassedValue))
		})

		It("returns error if creating a row fails", func() {
			storageKeysLookup.Metadata = fakeMetadata
			repository.CreateErr = fakes.FakeError
			diff := utils.PersistedStorageDiff{StorageDiffInput: utils.StorageDiffInput{StorageValue: rawValue.Hash()}}

			err := t.Execute(diff)

			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(fakes.FakeError))
		})
	})
})
