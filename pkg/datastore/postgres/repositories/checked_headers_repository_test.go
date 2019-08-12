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

package repositories_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/vulcanize/vulcanizedb/pkg/core"
	"github.com/vulcanize/vulcanizedb/pkg/datastore"
	"github.com/vulcanize/vulcanizedb/pkg/datastore/postgres"
	"github.com/vulcanize/vulcanizedb/pkg/datastore/postgres/repositories"
	"github.com/vulcanize/vulcanizedb/pkg/fakes"
	"github.com/vulcanize/vulcanizedb/test_config"
	"math/rand"
)

var _ = Describe("Checked Headers repository", func() {
	var (
		db   *postgres.DB
		repo datastore.CheckedHeadersRepository
	)

	Describe("MarkHeaderChecked", func() {
		BeforeEach(func() {
			db = test_config.NewTestDB(test_config.NewTestNode())
			test_config.CleanTestDB(db)
			repo = repositories.NewCheckedHeadersRepository(db)
		})

		It("marks passed header as checked on insert", func() {
			headerRepository := repositories.NewHeaderRepository(db)
			headerID, headerErr := headerRepository.CreateOrUpdateHeader(fakes.FakeHeader)
			Expect(headerErr).NotTo(HaveOccurred())

			err := repo.MarkHeaderChecked(headerID)

			Expect(err).NotTo(HaveOccurred())
			var checkedCount int
			fetchErr := db.Get(&checkedCount, `SELECT check_count FROM public.checked_headers WHERE header_id = $1`, headerID)
			Expect(fetchErr).NotTo(HaveOccurred())
			Expect(checkedCount).To(Equal(1))
		})

		It("increments check count on update", func() {
			headerRepository := repositories.NewHeaderRepository(db)
			headerID, headerErr := headerRepository.CreateOrUpdateHeader(fakes.FakeHeader)
			Expect(headerErr).NotTo(HaveOccurred())

			insertErr := repo.MarkHeaderChecked(headerID)
			Expect(insertErr).NotTo(HaveOccurred())

			updateErr := repo.MarkHeaderChecked(headerID)
			Expect(updateErr).NotTo(HaveOccurred())

			var checkedCount int
			fetchErr := db.Get(&checkedCount, `SELECT check_count FROM public.checked_headers WHERE header_id = $1`, headerID)
			Expect(fetchErr).NotTo(HaveOccurred())
			Expect(checkedCount).To(Equal(2))
		})
	})

	Describe("MissingHeaders", func() {
		var (
			headerRepository      datastore.HeaderRepository
			startingBlockNumber   int64
			endingBlockNumber     int64
			middleBlockNumber     int64
			outOfRangeBlockNumber int64
			blockNumbers          []int64
			headerIDs             []int64
			err                   error
			uncheckedCheckCount   = int64(1)
			recheckCheckCount     = int64(2)
		)

		BeforeEach(func() {
			db = test_config.NewTestDB(test_config.NewTestNode())
			test_config.CleanTestDB(db)
			headerRepository = repositories.NewHeaderRepository(db)
			repo = repositories.NewCheckedHeadersRepository(db)

			startingBlockNumber = rand.Int63()
			middleBlockNumber = startingBlockNumber + 1
			endingBlockNumber = startingBlockNumber + 2
			outOfRangeBlockNumber = endingBlockNumber + 1

			blockNumbers = []int64{startingBlockNumber, middleBlockNumber, endingBlockNumber, outOfRangeBlockNumber}

			headerIDs = []int64{}
			for _, n := range blockNumbers {
				headerID, err := headerRepository.CreateOrUpdateHeader(fakes.GetFakeHeader(n))
				headerIDs = append(headerIDs, headerID)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		Describe("when ending block is specified", func() {
			It("excludes headers that are out of range", func() {
				headers, err := repo.MissingHeaders(startingBlockNumber, endingBlockNumber, uncheckedCheckCount)

				Expect(err).NotTo(HaveOccurred())
				// doesn't include outOfRangeBlockNumber
				Expect(len(headers)).To(Equal(3))
				Expect(headers[0].BlockNumber).To(Or(Equal(startingBlockNumber), Equal(endingBlockNumber), Equal(middleBlockNumber)))
				Expect(headers[1].BlockNumber).To(Or(Equal(startingBlockNumber), Equal(endingBlockNumber), Equal(middleBlockNumber)))
				Expect(headers[2].BlockNumber).To(Or(Equal(startingBlockNumber), Equal(endingBlockNumber), Equal(middleBlockNumber)))
			})

			It("excludes headers that have been checked more than the check count", func() {
				_, err = db.Exec(`INSERT INTO public.checked_headers (header_id) VALUES ($1)`, headerIDs[1])
				Expect(err).NotTo(HaveOccurred())

				headers, err := repo.MissingHeaders(startingBlockNumber, endingBlockNumber, uncheckedCheckCount)

				Expect(err).NotTo(HaveOccurred())
				// doesn't include middleBlockNumber
				Expect(len(headers)).To(Equal(2))
				Expect(headers[0].BlockNumber).To(Or(Equal(startingBlockNumber), Equal(endingBlockNumber)))
				Expect(headers[1].BlockNumber).To(Or(Equal(startingBlockNumber), Equal(endingBlockNumber)))
			})

			It("does not exclude headers that have been checked less than the check count", func() {
				_, err = db.Exec(`INSERT INTO public.checked_headers (header_id) VALUES ($1)`, headerIDs[1])
				Expect(err).NotTo(HaveOccurred())

				headers, err := repo.MissingHeaders(startingBlockNumber, endingBlockNumber, recheckCheckCount)

				Expect(err).NotTo(HaveOccurred())
				Expect(len(headers)).To(Equal(3))
				Expect(headers[0].BlockNumber).To(Or(Equal(startingBlockNumber), Equal(middleBlockNumber), Equal(endingBlockNumber)))
				Expect(headers[1].BlockNumber).To(Or(Equal(startingBlockNumber), Equal(middleBlockNumber), Equal(endingBlockNumber)))
				Expect(headers[2].BlockNumber).To(Or(Equal(startingBlockNumber), Equal(middleBlockNumber), Equal(endingBlockNumber)))
			})

			It("only returns headers associated with the current node", func() {
				dbTwo := test_config.NewTestDB(core.Node{ID: "second"})
				headerRepositoryTwo := repositories.NewHeaderRepository(dbTwo)
				repoTwo := repositories.NewCheckedHeadersRepository(dbTwo)
				for _, n := range blockNumbers {
					_, err = headerRepositoryTwo.CreateOrUpdateHeader(fakes.GetFakeHeader(n + 10))
					Expect(err).NotTo(HaveOccurred())
				}

				Expect(err).NotTo(HaveOccurred())
				nodeOneMissingHeaders, err := repo.MissingHeaders(startingBlockNumber, endingBlockNumber, uncheckedCheckCount)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(nodeOneMissingHeaders)).To(Equal(3))
				Expect(nodeOneMissingHeaders[0].BlockNumber).To(Or(Equal(startingBlockNumber), Equal(middleBlockNumber), Equal(endingBlockNumber)))
				Expect(nodeOneMissingHeaders[1].BlockNumber).To(Or(Equal(startingBlockNumber), Equal(middleBlockNumber), Equal(endingBlockNumber)))
				Expect(nodeOneMissingHeaders[2].BlockNumber).To(Or(Equal(startingBlockNumber), Equal(middleBlockNumber), Equal(endingBlockNumber)))

				nodeTwoMissingHeaders, err := repoTwo.MissingHeaders(startingBlockNumber, endingBlockNumber+10, uncheckedCheckCount)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(nodeTwoMissingHeaders)).To(Equal(3))
				Expect(nodeTwoMissingHeaders[0].BlockNumber).To(Or(Equal(startingBlockNumber+10), Equal(middleBlockNumber+10), Equal(endingBlockNumber+10)))
				Expect(nodeTwoMissingHeaders[1].BlockNumber).To(Or(Equal(startingBlockNumber+10), Equal(middleBlockNumber+10), Equal(endingBlockNumber+10)))
				Expect(nodeTwoMissingHeaders[2].BlockNumber).To(Or(Equal(startingBlockNumber+10), Equal(middleBlockNumber+10), Equal(endingBlockNumber+10)))
			})

			It("only returns 100 results to prevent blocking log delegation", func() {
				for n := outOfRangeBlockNumber + 1; n < outOfRangeBlockNumber+100; n++ {
					_, err := headerRepository.CreateOrUpdateHeader(fakes.GetFakeHeader(n))
					Expect(err).NotTo(HaveOccurred())
				}

				missingHeaders, err := repo.MissingHeaders(startingBlockNumber, endingBlockNumber+200, uncheckedCheckCount)

				Expect(err).NotTo(HaveOccurred())
				Expect(len(missingHeaders)).To(Equal(100))
			})
		})

		Describe("when ending block is -1", func() {
			var endingBlock = int64(-1)

			It("includes all non-checked headers when ending block is -1 ", func() {
				headers, err := repo.MissingHeaders(startingBlockNumber, endingBlock, uncheckedCheckCount)

				Expect(err).NotTo(HaveOccurred())
				Expect(len(headers)).To(Equal(4))
				Expect(headers[0].BlockNumber).To(Or(Equal(startingBlockNumber), Equal(endingBlockNumber), Equal(middleBlockNumber), Equal(outOfRangeBlockNumber)))
				Expect(headers[1].BlockNumber).To(Or(Equal(startingBlockNumber), Equal(endingBlockNumber), Equal(middleBlockNumber), Equal(outOfRangeBlockNumber)))
				Expect(headers[2].BlockNumber).To(Or(Equal(startingBlockNumber), Equal(endingBlockNumber), Equal(middleBlockNumber), Equal(outOfRangeBlockNumber)))
				Expect(headers[3].BlockNumber).To(Or(Equal(startingBlockNumber), Equal(endingBlockNumber), Equal(middleBlockNumber), Equal(outOfRangeBlockNumber)))
			})

			It("excludes headers that have been checked more than the check count", func() {
				_, err = db.Exec(`INSERT INTO public.checked_headers (header_id) VALUES ($1)`, headerIDs[1])
				Expect(err).NotTo(HaveOccurred())

				headers, err := repo.MissingHeaders(startingBlockNumber, endingBlock, uncheckedCheckCount)

				Expect(err).NotTo(HaveOccurred())
				// doesn't include middleBlockNumber
				Expect(len(headers)).To(Equal(3))
				Expect(headers[0].BlockNumber).To(Or(Equal(startingBlockNumber), Equal(endingBlockNumber), Equal(outOfRangeBlockNumber)))
				Expect(headers[1].BlockNumber).To(Or(Equal(startingBlockNumber), Equal(endingBlockNumber), Equal(outOfRangeBlockNumber)))
				Expect(headers[2].BlockNumber).To(Or(Equal(startingBlockNumber), Equal(endingBlockNumber), Equal(outOfRangeBlockNumber)))
			})

			It("does not exclude headers that have been checked less than the check count", func() {
				_, err = db.Exec(`INSERT INTO public.checked_headers (header_id) VALUES ($1)`, headerIDs[1])
				Expect(err).NotTo(HaveOccurred())

				headers, err := repo.MissingHeaders(startingBlockNumber, endingBlock, recheckCheckCount)

				Expect(err).NotTo(HaveOccurred())
				Expect(len(headers)).To(Equal(4))
				Expect(headers[0].BlockNumber).To(Or(Equal(startingBlockNumber), Equal(middleBlockNumber), Equal(endingBlockNumber), Equal(outOfRangeBlockNumber)))
				Expect(headers[1].BlockNumber).To(Or(Equal(startingBlockNumber), Equal(middleBlockNumber), Equal(endingBlockNumber), Equal(outOfRangeBlockNumber)))
				Expect(headers[2].BlockNumber).To(Or(Equal(startingBlockNumber), Equal(middleBlockNumber), Equal(endingBlockNumber), Equal(outOfRangeBlockNumber)))
				Expect(headers[3].BlockNumber).To(Or(Equal(startingBlockNumber), Equal(middleBlockNumber), Equal(endingBlockNumber), Equal(outOfRangeBlockNumber)))
			})

			It("only returns headers associated with the current node", func() {
				dbTwo := test_config.NewTestDB(core.Node{ID: "second"})
				headerRepositoryTwo := repositories.NewHeaderRepository(dbTwo)
				repoTwo := repositories.NewCheckedHeadersRepository(dbTwo)
				for _, n := range blockNumbers {
					_, err = headerRepositoryTwo.CreateOrUpdateHeader(fakes.GetFakeHeader(n + 10))
					Expect(err).NotTo(HaveOccurred())
				}

				Expect(err).NotTo(HaveOccurred())
				nodeOneMissingHeaders, err := repo.MissingHeaders(startingBlockNumber, endingBlock, uncheckedCheckCount)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(nodeOneMissingHeaders)).To(Equal(4))
				Expect(nodeOneMissingHeaders[0].BlockNumber).To(Or(Equal(startingBlockNumber), Equal(middleBlockNumber), Equal(endingBlockNumber), Equal(outOfRangeBlockNumber)))
				Expect(nodeOneMissingHeaders[1].BlockNumber).To(Or(Equal(startingBlockNumber), Equal(middleBlockNumber), Equal(endingBlockNumber), Equal(outOfRangeBlockNumber)))
				Expect(nodeOneMissingHeaders[2].BlockNumber).To(Or(Equal(startingBlockNumber), Equal(middleBlockNumber), Equal(endingBlockNumber), Equal(outOfRangeBlockNumber)))
				Expect(nodeOneMissingHeaders[3].BlockNumber).To(Or(Equal(startingBlockNumber), Equal(middleBlockNumber), Equal(endingBlockNumber), Equal(outOfRangeBlockNumber)))

				nodeTwoMissingHeaders, err := repoTwo.MissingHeaders(startingBlockNumber, endingBlock, uncheckedCheckCount)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(nodeTwoMissingHeaders)).To(Equal(4))
				Expect(nodeTwoMissingHeaders[0].BlockNumber).To(Or(Equal(startingBlockNumber+10), Equal(middleBlockNumber+10), Equal(endingBlockNumber+10), Equal(outOfRangeBlockNumber+10)))
				Expect(nodeTwoMissingHeaders[1].BlockNumber).To(Or(Equal(startingBlockNumber+10), Equal(middleBlockNumber+10), Equal(endingBlockNumber+10), Equal(outOfRangeBlockNumber+10)))
				Expect(nodeTwoMissingHeaders[2].BlockNumber).To(Or(Equal(startingBlockNumber+10), Equal(middleBlockNumber+10), Equal(endingBlockNumber+10), Equal(outOfRangeBlockNumber+10)))
				Expect(nodeTwoMissingHeaders[3].BlockNumber).To(Or(Equal(startingBlockNumber+10), Equal(middleBlockNumber+10), Equal(endingBlockNumber+10), Equal(outOfRangeBlockNumber+10)))
			})

			It("only returns 100 results to prevent blocking log delegation", func() {
				for n := outOfRangeBlockNumber + 1; n < outOfRangeBlockNumber+100; n++ {
					_, err := headerRepository.CreateOrUpdateHeader(fakes.GetFakeHeader(n))
					Expect(err).NotTo(HaveOccurred())
				}

				missingHeaders, err := repo.MissingHeaders(startingBlockNumber, endingBlock, uncheckedCheckCount)

				Expect(err).NotTo(HaveOccurred())
				Expect(len(missingHeaders)).To(Equal(100))
			})
		})

	})
})
