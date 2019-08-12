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

package fakes

type MockFullSyncBlockRetriever struct {
	FirstBlock      int64
	FirstBlockErr   error
	MostRecentBlock int64
}

func (retriever *MockFullSyncBlockRetriever) RetrieveFirstBlock(contractAddr string) (int64, error) {
	return retriever.FirstBlock, retriever.FirstBlockErr
}

func (retriever *MockFullSyncBlockRetriever) RetrieveMostRecentBlock() (int64, error) {
	return retriever.MostRecentBlock, nil
}
