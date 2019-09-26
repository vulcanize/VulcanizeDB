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

package history

import (
	log "github.com/sirupsen/logrus"

	"github.com/vulcanize/vulcanizedb/pkg/core"
	"github.com/vulcanize/vulcanizedb/pkg/datastore"
)

func PopulateMissingHeaders(blockChain core.BlockChain, headerRepository datastore.HeaderRepository, startingBlockNumber int64) (int, error) {
	lastBlock, err := blockChain.LastBlock()
	if err != nil {
		log.Error("PopulateMissingHeaders: Error getting last block: ", err)
		return 0, err
	}

	blockNumbers, err := headerRepository.MissingBlockNumbers(startingBlockNumber, lastBlock.Int64(), blockChain.Node().ID)
	if err != nil {
		log.Error("PopulateMissingHeaders: Error getting missing block numbers: ", err)
		return 0, err
	} else if len(blockNumbers) == 0 {
		return 0, nil
	}

	log.Debug(getBlockRangeString(blockNumbers))
	_, err = RetrieveAndUpdateHeaders(blockChain, headerRepository, blockNumbers)
	if err != nil {
		log.Error("PopulateMissingHeaders: Error getting/updating headers: ", err)
		return 0, err
	}
	return len(blockNumbers), nil
}

func RetrieveAndUpdateHeaders(blockChain core.BlockChain, headerRepository datastore.HeaderRepository, blockNumbers []int64) (int, error) {
	headers, getErr := blockChain.GetHeadersByNumbers(blockNumbers)
	if getErr != nil {
		return 0, getErr
	}
	for _, header := range headers {
		_, insertErr := headerRepository.CreateOrUpdateHeader(header)
		if insertErr != nil {
			return 0, insertErr
		}
	}
	return len(blockNumbers), nil
}
