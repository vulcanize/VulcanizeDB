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

package converter

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	gethTypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/vulcanize/vulcanizedb/pkg/contract_watcher/shared/contract"
	"github.com/vulcanize/vulcanizedb/pkg/contract_watcher/shared/types"
)

// ConverterInterface is the interface for converting geth logs to our custom log type
type ConverterInterface interface {
	Convert(logs []gethTypes.Log, event types.Event, headerID int64) ([]types.Log, error)
	ConvertBatch(logs []gethTypes.Log, events map[string]types.Event, headerID int64) (map[string][]types.Log, error)
	Update(info *contract.Contract)
}

// Converter is the underlying struct for the ConverterInterface
type Converter struct {
	ContractInfo *contract.Contract
}

// Update is used to configure the converter with a specific contract
func (c *Converter) Update(info *contract.Contract) {
	c.ContractInfo = info
}

// Convert the given watched event log into a types.Log for the given event
func (c *Converter) Convert(logs []gethTypes.Log, event types.Event, headerID int64) ([]types.Log, error) {
	boundContract := bind.NewBoundContract(common.HexToAddress(c.ContractInfo.Address), c.ContractInfo.ParsedAbi, nil, nil, nil)
	returnLogs := make([]types.Log, 0, len(logs))
	for _, log := range logs {
		values := make(map[string]interface{})
		for _, field := range event.Fields {
			var i interface{}
			values[field.Name] = i
		}

		err := boundContract.UnpackLogIntoMap(values, event.Name, log)
		if err != nil {
			return nil, err
		}

		strValues := make(map[string]string, len(values))
		seenAddrs := make([]interface{}, 0, len(values))
		seenHashes := make([]interface{}, 0, len(values))
		for fieldName, input := range values {
			// Postgres cannot handle custom types, resolve everything to strings
			switch input.(type) {
			case *big.Int:
				b := input.(*big.Int)
				strValues[fieldName] = b.String()
			case common.Address:
				a := input.(common.Address)
				strValues[fieldName] = a.String()
				seenAddrs = append(seenAddrs, a)
			case common.Hash:
				h := input.(common.Hash)
				strValues[fieldName] = h.String()
				seenHashes = append(seenHashes, h)
			case string:
				strValues[fieldName] = input.(string)
			case bool:
				strValues[fieldName] = strconv.FormatBool(input.(bool))
			case []byte:
				b := input.([]byte)
				strValues[fieldName] = hexutil.Encode(b)
				if len(b) == 32 {
					seenHashes = append(seenHashes, common.HexToHash(strValues[fieldName]))
				}
			case uint8:
				u := input.(uint8)
				strValues[fieldName] = strconv.Itoa(int(u))
			case [32]uint8:
				raw := input.([32]uint8)
				converted := convertUintSliceToHash(raw)
				strValues[fieldName] = converted.String()
				seenHashes = append(seenHashes, converted)
			default:
				return nil, fmt.Errorf("error: unhandled abi type %T", input)
			}
		}

		// Only hold onto logs that pass our address filter, if any
		if c.ContractInfo.PassesEventFilter(strValues) {
			raw, err := json.Marshal(log)
			if err != nil {
				return nil, err
			}

			returnLogs = append(returnLogs, types.Log{
				LogIndex:         log.Index,
				Values:           strValues,
				Raw:              raw,
				TransactionIndex: log.TxIndex,
				ID:               headerID,
			})

			// Cache emitted values if their caching is turned on
			if c.ContractInfo.EmittedAddrs != nil {
				c.ContractInfo.AddEmittedAddr(seenAddrs...)
			}
			if c.ContractInfo.EmittedHashes != nil {
				c.ContractInfo.AddEmittedHash(seenHashes...)
			}
		}
	}

	return returnLogs, nil
}

// ConvertBatch converts the given watched event logs into types.Logs; returns a map of event names to a slice of their converted logs
func (c *Converter) ConvertBatch(logs []gethTypes.Log, events map[string]types.Event, headerID int64) (map[string][]types.Log, error) {
	boundContract := bind.NewBoundContract(common.HexToAddress(c.ContractInfo.Address), c.ContractInfo.ParsedAbi, nil, nil, nil)
	eventsToLogs := make(map[string][]types.Log)
	for _, event := range events {
		eventsToLogs[event.Name] = make([]types.Log, 0, len(logs))
		// Iterate through all event logs
		for _, log := range logs {
			// If the log is of this event type, process it as such
			if event.Sig() == log.Topics[0] {
				values := make(map[string]interface{})
				err := boundContract.UnpackLogIntoMap(values, event.Name, log)
				if err != nil {
					return nil, err
				}
				// Postgres cannot handle custom types, so we will resolve everything to strings
				strValues := make(map[string]string, len(values))
				// Keep track of addresses and hashes emitted from events
				seenAddrs := make([]interface{}, 0, len(values))
				seenHashes := make([]interface{}, 0, len(values))
				for fieldName, input := range values {
					switch input.(type) {
					case *big.Int:
						b := input.(*big.Int)
						strValues[fieldName] = b.String()
					case common.Address:
						a := input.(common.Address)
						strValues[fieldName] = a.String()
						seenAddrs = append(seenAddrs, a)
					case common.Hash:
						h := input.(common.Hash)
						strValues[fieldName] = h.String()
						seenHashes = append(seenHashes, h)
					case string:
						strValues[fieldName] = input.(string)
					case bool:
						strValues[fieldName] = strconv.FormatBool(input.(bool))
					case []byte:
						b := input.([]byte)
						strValues[fieldName] = hexutil.Encode(b)
						if len(b) == 32 { // collect byte arrays of size 32 as hashes
							seenHashes = append(seenHashes, common.BytesToHash(b))
						}
					case uint8:
						u := input.(uint8)
						strValues[fieldName] = strconv.Itoa(int(u))
					case [32]uint8:
						raw := input.([32]uint8)
						converted := convertUintSliceToHash(raw)
						strValues[fieldName] = converted.String()
						seenHashes = append(seenHashes, converted)
					default:
						return nil, fmt.Errorf("error: unhandled abi type %T", input)
					}
				}

				// Only hold onto logs that pass our argument filter, if any
				if c.ContractInfo.PassesEventFilter(strValues) {
					raw, err := json.Marshal(log)
					if err != nil {
						return nil, err
					}

					eventsToLogs[event.Name] = append(eventsToLogs[event.Name], types.Log{
						LogIndex:         log.Index,
						Values:           strValues,
						Raw:              raw,
						TransactionIndex: log.TxIndex,
						ID:               headerID,
					})

					// Cache emitted values that pass the argument filter if their caching is turned on
					if c.ContractInfo.EmittedAddrs != nil {
						c.ContractInfo.AddEmittedAddr(seenAddrs...)
					}
					if c.ContractInfo.EmittedHashes != nil {
						c.ContractInfo.AddEmittedHash(seenHashes...)
					}
				}
			}
		}
	}

	return eventsToLogs, nil
}

func convertUintSliceToHash(raw [32]uint8) common.Hash {
	var asBytes []byte
	for _, u := range raw {
		asBytes = append(asBytes, u)
	}
	return common.BytesToHash(asBytes)
}
