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

package logs

import (
	"errors"
	"github.com/ethereum/go-ethereum/common"
	"github.com/sirupsen/logrus"
	"github.com/vulcanize/vulcanizedb/libraries/shared/constants"
	"github.com/vulcanize/vulcanizedb/libraries/shared/fetcher"
	"github.com/vulcanize/vulcanizedb/libraries/shared/transactions"
	"github.com/vulcanize/vulcanizedb/libraries/shared/transformer"
	"github.com/vulcanize/vulcanizedb/pkg/core"
	"github.com/vulcanize/vulcanizedb/pkg/datastore"
)

var ErrNoWatchedAddresses = errors.New("no watched addresses configured in the log extractor")

type ILogExtractor interface {
	AddTransformerConfig(config transformer.EventTransformerConfig)
	ExtractLogs(recheckHeaders constants.TransformerExecution, errs chan error, missingHeadersFound chan bool)
}

type LogExtractor struct {
	Addresses                []common.Address
	CheckedHeadersRepository datastore.CheckedHeadersRepository
	Fetcher                  fetcher.ILogFetcher
	LogRepository            datastore.HeaderSyncLogRepository
	StartingBlock            *int64
	Syncer                   transactions.ITransactionsSyncer
	Topics                   []common.Hash
}

// Add additional logs to extract
func (extractor *LogExtractor) AddTransformerConfig(config transformer.EventTransformerConfig) {
	if extractor.StartingBlock == nil {
		extractor.StartingBlock = &config.StartingBlockNumber
	} else if earlierStartingBlockNumber(config.StartingBlockNumber, *extractor.StartingBlock) {
		extractor.StartingBlock = &config.StartingBlockNumber
	}

	addresses := transformer.HexStringsToAddresses(config.ContractAddresses)
	extractor.Addresses = append(extractor.Addresses, addresses...)
	extractor.Topics = append(extractor.Topics, common.HexToHash(config.Topic))
}

// Fetch and persist watched logs
func (extractor LogExtractor) ExtractLogs(recheckHeaders constants.TransformerExecution, errs chan error, missingHeadersFound chan bool) {
	if len(extractor.Addresses) < 1 {
		logrus.Errorf("error extracting logs: %s", ErrNoWatchedAddresses.Error())
		errs <- ErrNoWatchedAddresses
		return
	}

	missingHeaders, missingHeadersErr := extractor.CheckedHeadersRepository.MissingHeaders(*extractor.StartingBlock, -1, getCheckCount(recheckHeaders))
	if missingHeadersErr != nil {
		logrus.Errorf("error fetching missing headers: %s", missingHeadersErr)
		errs <- missingHeadersErr
		return
	}

	if len(missingHeaders) < 1 {
		missingHeadersFound <- false
		return
	}

	for _, header := range missingHeaders {
		logs, fetchLogsErr := extractor.Fetcher.FetchLogs(extractor.Addresses, extractor.Topics, header)
		if fetchLogsErr != nil {
			logError("error fetching logs for header: %s", fetchLogsErr, header)
			errs <- fetchLogsErr
			return
		}

		if len(logs) > 0 {
			transactionsSyncErr := extractor.Syncer.SyncTransactions(header.Id, logs)
			if transactionsSyncErr != nil {
				logError("error syncing transactions: %s", transactionsSyncErr, header)
				errs <- transactionsSyncErr
				return
			}

			createLogsErr := extractor.LogRepository.CreateHeaderSyncLogs(header.Id, logs)
			if createLogsErr != nil {
				logError("error persisting logs: %s", createLogsErr, header)
				errs <- createLogsErr
				return
			}
		}

		markHeaderCheckedErr := extractor.CheckedHeadersRepository.MarkHeaderChecked(header.Id)
		if markHeaderCheckedErr != nil {
			logError("error marking header checked: %s", markHeaderCheckedErr, header)
			errs <- markHeaderCheckedErr
		}
	}
	missingHeadersFound <- true
}

func earlierStartingBlockNumber(transformerBlock, watcherBlock int64) bool {
	return transformerBlock < watcherBlock
}

func logError(description string, err error, header core.Header) {
	logrus.WithFields(logrus.Fields{
		"headerId":    header.Id,
		"headerHash":  header.Hash,
		"blockNumber": header.BlockNumber,
	}).Errorf(description, err.Error())
}

func getCheckCount(recheckHeaders constants.TransformerExecution) int64 {
	if recheckHeaders == constants.HeaderMissing {
		return 1
	} else {
		return constants.RecheckHeaderCap
	}
}
