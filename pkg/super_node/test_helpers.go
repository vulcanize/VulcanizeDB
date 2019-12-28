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

package super_node

import (
	"bytes"

	. "github.com/onsi/gomega"

	"github.com/vulcanize/vulcanizedb/pkg/config"
	"github.com/vulcanize/vulcanizedb/pkg/core"
	"github.com/vulcanize/vulcanizedb/pkg/datastore/postgres"
)

// SetupDB is use to setup a db for super node tests
func SetupDB() (*postgres.DB, error) {
	return postgres.NewDB(config.Database{
		Hostname: "localhost",
		Name:     "vulcanize_testing",
		Port:     5432,
	}, core.Node{})
}

// TearDownDB is used to tear down the super node dbs after tests
func TearDownDB(db *postgres.DB) {
	tx, err := db.Beginx()
	Expect(err).NotTo(HaveOccurred())

	_, err = tx.Exec(`DELETE FROM header_cids`)
	Expect(err).NotTo(HaveOccurred())
	_, err = tx.Exec(`DELETE FROM transaction_cids`)
	Expect(err).NotTo(HaveOccurred())
	_, err = tx.Exec(`DELETE FROM receipt_cids`)
	Expect(err).NotTo(HaveOccurred())
	_, err = tx.Exec(`DELETE FROM state_cids`)
	Expect(err).NotTo(HaveOccurred())
	_, err = tx.Exec(`DELETE FROM storage_cids`)
	Expect(err).NotTo(HaveOccurred())
	_, err = tx.Exec(`DELETE FROM blocks`)
	Expect(err).NotTo(HaveOccurred())

	err = tx.Commit()
	Expect(err).NotTo(HaveOccurred())
}

// ListContainsString used to check if a list of strings contains a particular string
func ListContainsString(sss []string, s string) bool {
	for _, str := range sss {
		if s == str {
			return true
		}
	}
	return false
}

// ListContainsBytes used to check if a list of byte arrays contains a particular byte array
func ListContainsBytes(bbb [][]byte, b []byte) bool {
	for _, by := range bbb {
		if bytes.Equal(by, b) {
			return true
		}
	}
	return false
}

// ListContainsRange used to check if a list of [2]uint64 contains a particula [2]uint64
func ListContainsRange(rangeList [][2]uint64, rng [2]uint64) bool {
	for _, rangeInList := range rangeList {
		if rangeInList == rng {
			return true
		}
	}
	return false
}
