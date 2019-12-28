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

package utils

import (
	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
	"math/big"
	"os"
	"path/filepath"

	"github.com/vulcanize/vulcanizedb/pkg/config"
	"github.com/vulcanize/vulcanizedb/pkg/core"
	"github.com/vulcanize/vulcanizedb/pkg/datastore/postgres"
	"github.com/vulcanize/vulcanizedb/pkg/eth"
)

func LoadPostgres(database config.Database, node core.Node) postgres.DB {
	db, err := postgres.NewDB(database, node)
	if err != nil {
		logrus.Fatal("Error loading postgres: ", err)
	}
	return *db
}

func ReadAbiFile(abiFilepath string) string {
	abiFilepath = AbsFilePath(abiFilepath)
	abi, err := eth.ReadAbiFile(abiFilepath)
	if err != nil {
		logrus.Fatalf("Error reading ABI file at \"%s\"\n %v", abiFilepath, err)
	}
	return abi
}

func AbsFilePath(filePath string) string {
	if !filepath.IsAbs(filePath) {
		cwd, _ := os.Getwd()
		filePath = filepath.Join(cwd, filePath)
	}
	return filePath
}

func GetAbi(abiFilepath string, contractHash string, network string) string {
	var contractAbiString string
	if abiFilepath != "" {
		contractAbiString = ReadAbiFile(abiFilepath)
	} else {
		url := eth.GenURL(network)
		etherscan := eth.NewEtherScanClient(url)
		logrus.Printf("No ABI supplied. Retrieving ABI from Etherscan: %s", url)
		contractAbiString, _ = etherscan.GetAbi(contractHash)
	}
	_, err := eth.ParseAbi(contractAbiString)
	if err != nil {
		logrus.Fatalln("Invalid ABI: ", err)
	}
	return contractAbiString
}

func RequestedBlockNumber(blockNumber *int64) *big.Int {
	var _blockNumber *big.Int
	if *blockNumber == -1 {
		_blockNumber = nil
	} else {
		_blockNumber = big.NewInt(*blockNumber)
	}
	return _blockNumber
}

func RollbackAndLogFailure(tx *sqlx.Tx, txErr error, fieldName string) {
	rollbackErr := tx.Rollback()
	if rollbackErr != nil {
		logrus.WithFields(logrus.Fields{"rollbackErr": rollbackErr, "txErr": txErr}).
			Warnf("failed to rollback transaction after failing to insert %s", fieldName)
	}
}
