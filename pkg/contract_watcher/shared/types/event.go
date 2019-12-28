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

package types

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// Event is our custom event type
type Event struct {
	Name      string
	Anonymous bool
	Fields    []Field
}

// Field is our custom event field type which associates a postgres type with the field
type Field struct {
	abi.Argument        // Name, Type, Indexed
	PgType       string // Holds type used when committing data held in this field to postgres
}

// Log is used to hold instance of an event log data
type Log struct {
	ID     int64             // VulcanizeIdLog for full sync and header ID for header sync contract watcher
	Values map[string]string // Map of event input names to their values

	// Used for full sync only
	Block int64
	Tx    string

	// Used for headerSync only
	LogIndex         uint
	TransactionIndex uint
	Raw              []byte // json.Unmarshalled byte array of geth/core/types.Log{}
}

// NewEvent unpacks abi.Event into our custom Event struct
func NewEvent(e abi.Event) Event {
	fields := make([]Field, len(e.Inputs))
	for i, input := range e.Inputs {
		fields[i] = Field{}
		fields[i].Name = input.Name
		fields[i].Type = input.Type
		fields[i].Indexed = input.Indexed
		// Fill in pg type based on abi type
		switch fields[i].Type.T {
		case abi.HashTy, abi.AddressTy:
			fields[i].PgType = "CHARACTER VARYING(66)"
		case abi.IntTy, abi.UintTy:
			fields[i].PgType = "NUMERIC"
		case abi.BoolTy:
			fields[i].PgType = "BOOLEAN"
		case abi.BytesTy, abi.FixedBytesTy:
			fields[i].PgType = "BYTEA"
		case abi.ArrayTy:
			fields[i].PgType = "TEXT[]"
		case abi.FixedPointTy:
			fields[i].PgType = "MONEY" // use shopspring/decimal for fixed point numbers in go and money type in postgres?
		default:
			fields[i].PgType = "TEXT"
		}
	}

	return Event{
		Name:      e.Name,
		Anonymous: e.Anonymous,
		Fields:    fields,
	}
}

// Sig returns the hash signature for an event
func (e Event) Sig() common.Hash {
	types := make([]string, len(e.Fields))

	for i, input := range e.Fields {
		types[i] = input.Type.String()
	}

	return crypto.Keccak256Hash([]byte(fmt.Sprintf("%v(%v)", e.Name, strings.Join(types, ","))))
}
