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

package retriever

import (
	"database/sql"

	"github.com/vulcanize/vulcanizedb/libraries/shared/repository"
	"github.com/vulcanize/vulcanizedb/pkg/datastore/postgres"
)

// BlockRetriever is used to retrieve the first block for a given contract and the most recent block
// It requires a vDB synced database with blocks, transactions, receipts, and logs
type BlockRetriever interface {
	RetrieveFirstBlock(contractAddr string) (int64, error)
	RetrieveMostRecentBlock() (int64, error)
}

type blockRetriever struct {
	db *postgres.DB
}

// NewBlockRetriever returns a new BlockRetriever
func NewBlockRetriever(db *postgres.DB) BlockRetriever {
	return &blockRetriever{
		db: db,
	}
}

// RetrieveFirstBlock fetches the block number for the earliest block in the db
// Tries both methods of finding the first block, with the receipt method taking precedence
func (r *blockRetriever) RetrieveFirstBlock(contractAddr string) (int64, error) {
	i, err := r.retrieveFirstBlockFromReceipts(contractAddr)
	if err != nil {
		if err == sql.ErrNoRows {
			i, err = r.retrieveFirstBlockFromLogs(contractAddr)
		}
		return i, err
	}

	return i, err
}

// For some contracts the contract creation transaction receipt doesn't have the contract address so this doesn't work (e.g. Sai)
func (r *blockRetriever) retrieveFirstBlockFromReceipts(contractAddr string) (int64, error) {
	var firstBlock int64
	addressID, getAddressErr := repository.GetOrCreateAddress(r.db, contractAddr)
	if getAddressErr != nil {
		return firstBlock, getAddressErr
	}
	err := r.db.Get(
		&firstBlock,
		`SELECT number FROM eth_blocks
		       WHERE id = (SELECT block_id FROM full_sync_receipts
                           WHERE contract_address_id = $1
		                   ORDER BY block_id ASC
					       LIMIT 1)`,
		addressID,
	)

	return firstBlock, err
}

// In which case this servers as a heuristic to find the first block by finding the first contract event log
func (r *blockRetriever) retrieveFirstBlockFromLogs(contractAddr string) (int64, error) {
	var firstBlock int
	err := r.db.Get(
		&firstBlock,
		"SELECT block_number FROM full_sync_logs WHERE lower(address) = $1 ORDER BY block_number ASC LIMIT 1",
		contractAddr,
	)

	return int64(firstBlock), err
}

// RetrieveMostRecentBlock retrieves the most recent block number in vDB
func (r *blockRetriever) RetrieveMostRecentBlock() (int64, error) {
	var lastBlock int64
	err := r.db.Get(
		&lastBlock,
		"SELECT number FROM eth_blocks ORDER BY number DESC LIMIT 1",
	)

	return lastBlock, err
}
