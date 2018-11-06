package vat_tune

import "github.com/vulcanize/vulcanizedb/pkg/transformers/shared"

var VatTuneConfig = shared.TransformerConfig{
	TransformerName:     shared.VatTuneLabel,
	ContractAddresses:   []string{shared.VatContractAddress},
	ContractAbi:         shared.VatABI,
	Topic:               shared.VatTuneSignature,
	StartingBlockNumber: 0,
	EndingBlockNumber:   10000000,
}