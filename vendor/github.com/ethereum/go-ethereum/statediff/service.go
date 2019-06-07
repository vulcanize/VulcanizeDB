// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package statediff

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rpc"
)

type blockChain interface {
	SubscribeChainEvent(ch chan<- core.ChainEvent) event.Subscription
	GetBlockByHash(hash common.Hash) *types.Block
	AddToStateDiffProcessedCollection(hash common.Hash)
}

// IService is the state-diffing service interface
type IService interface {
	// APIs(), Protocols(), Start() and Stop()
	node.Service
	// Main event loop for processing state diffs
	Loop(chainEventCh chan core.ChainEvent)
	// Method to subscribe to receive state diff processing output
	Subscribe(id rpc.ID, sub chan<- Payload, quitChan chan<- bool)
	// Method to unsubscribe from state diff processing
	Unsubscribe(id rpc.ID) error
}

// Service is the underlying struct for the state diffing service
type Service struct {
	// Used to sync access to the Subscriptions
	sync.Mutex
	// Used to build the state diff objects
	Builder Builder
	// Used to subscribe to chain events (blocks)
	BlockChain blockChain
	// Used to signal shutdown of the service
	QuitChan chan bool
	// A mapping of rpc.IDs to their subscription channels
	Subscriptions map[rpc.ID]Subscription
}

// Subscription struct holds our subscription channels
type Subscription struct {
	PayloadChan chan<- Payload
	QuitChan    chan<- bool
}

// Payload packages the data to send to StateDiffingService subscriptions
type Payload struct {
	BlockRlp     []byte `json:"blockRlp"     gencodec:"required"`
	StateDiffRlp []byte `json:"stateDiffRlp" gencodec:"required"`
	Err          error  `json:"error"`
}

// NewStateDiffService creates a new StateDiffingService
func NewStateDiffService(db ethdb.Database, blockChain *core.BlockChain) (*Service, error) {
	return &Service{
		Mutex:         sync.Mutex{},
		BlockChain:    blockChain,
		Builder:       NewBuilder(db, blockChain),
		QuitChan:      make(chan bool),
		Subscriptions: make(map[rpc.ID]Subscription),
	}, nil
}

// Protocols exports the services p2p protocols, this service has none
func (sds *Service) Protocols() []p2p.Protocol {
	return []p2p.Protocol{}
}

// APIs returns the RPC descriptors the StateDiffingService offers
func (sds *Service) APIs() []rpc.API {
	return []rpc.API{
		{
			Namespace: APIName,
			Version:   APIVersion,
			Service:   NewPublicStateDiffAPI(sds),
			Public:    true,
		},
	}
}

// Loop is the main processing method
func (sds *Service) Loop(chainEventCh chan core.ChainEvent) {

	chainEventSub := sds.BlockChain.SubscribeChainEvent(chainEventCh)
	defer chainEventSub.Unsubscribe()

	blocksCh := make(chan *types.Block, 10)
	errCh := chainEventSub.Err()

	go func() {
	HandleChainEventChLoop:
		for {
			select {
			//Notify chain event channel of events
			case chainEvent := <-chainEventCh:
				log.Debug("Event received from chainEventCh", "event", chainEvent)
				blocksCh <- chainEvent.Block
				//if node stopped
			case err := <-errCh:
				log.Warn("Error from chain event subscription, breaking loop.", "error", err)
				close(sds.QuitChan)
				break HandleChainEventChLoop
			case <-sds.QuitChan:
				break HandleChainEventChLoop
			}
		}
	}()

	//loop through chain events until no more
HandleBlockChLoop:
	for {
		select {
		case block := <-blocksCh:
			currentBlock := block
			parentHash := currentBlock.ParentHash()
			parentBlock := sds.BlockChain.GetBlockByHash(parentHash)
			if parentBlock == nil {
				log.Error("Parent block is nil, skipping this block",
					"parent block hash", parentHash.String(),
					"current block number", currentBlock.Number())
				break HandleBlockChLoop
			}

			stateDiff, err := sds.Builder.BuildStateDiff(parentBlock.Root(), currentBlock.Root(), currentBlock.Number().Int64(), currentBlock.Hash())
			if err != nil {
				log.Error("Error building statediff", "block number", currentBlock.Number(), "error", err)
			}
			rlpBuff := new(bytes.Buffer)
			currentBlock.EncodeRLP(rlpBuff)
			blockRlp := rlpBuff.Bytes()
			stateDiffRlp, _ := rlp.EncodeToBytes(stateDiff)
			payload := Payload{
				BlockRlp:     blockRlp,
				StateDiffRlp: stateDiffRlp,
				Err:          err,
			}
			// If we have any websocket subscription listening in, send the data to them
			sds.send(payload)
		case <-sds.QuitChan:
			log.Debug("Quitting the statediff block channel")
			sds.close()
			return
		}
	}
}

// Subscribe is used by the API to subscribe to the StateDiffingService loop
func (sds *Service) Subscribe(id rpc.ID, sub chan<- Payload, quitChan chan<- bool) {
	log.Info("Subscribing to the statediff service")
	sds.Lock()
	sds.Subscriptions[id] = Subscription{
		PayloadChan: sub,
		QuitChan:    quitChan,
	}
	sds.Unlock()
}

// Unsubscribe is used to unsubscribe to the StateDiffingService loop
func (sds *Service) Unsubscribe(id rpc.ID) error {
	log.Info("Unsubscribing from the statediff service")
	sds.Lock()
	_, ok := sds.Subscriptions[id]
	if !ok {
		return fmt.Errorf("cannot unsubscribe; subscription for id %s does not exist", id)
	}
	delete(sds.Subscriptions, id)
	sds.Unlock()
	return nil
}

// Start is used to begin the StateDiffingService
func (sds *Service) Start(*p2p.Server) error {
	log.Info("Starting statediff service")

	chainEventCh := make(chan core.ChainEvent, 10)
	go sds.Loop(chainEventCh)

	return nil
}

// Stop is used to close down the StateDiffingService
func (sds *Service) Stop() error {
	log.Info("Stopping statediff service")
	close(sds.QuitChan)
	return nil
}

// send is used to fan out and serve a payload to any subscriptions
func (sds *Service) send(payload Payload) {
	sds.Lock()
	for id, sub := range sds.Subscriptions {
		select {
		case sub.PayloadChan <- payload:
			log.Info("sending state diff payload to subscription %s", id)
		default:
			log.Info("unable to send payload to subscription %s; channel has no receiver", id)
		}
	}
	sds.Unlock()
}

// close is used to close all listening subscriptions
func (sds *Service) close() {
	sds.Lock()
	for id, sub := range sds.Subscriptions {
		select {
		case sub.QuitChan <- true:
			delete(sds.Subscriptions, id)
			log.Info("closing subscription %s", id)
		default:
			log.Info("unable to close subscription %s; channel has no receiver", id)
		}
	}
	sds.Unlock()
}
