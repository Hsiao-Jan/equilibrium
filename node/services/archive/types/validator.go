package types

/*
// BlockValidator is responsible for validating block headers and
// processed state. BlockValidator implements Validator.
type BlockValidator struct {
	network *params.Network  // Chain configuration options
	bc      *BlockChain      // Canonical block chain
	engine  consensus.Engine // Consensus engine used for validating
}

// NewBlockValidator returns a new block validator which is safe for re-use
func NewBlockValidator(network *params.Network, blockchain *BlockChain, engine consensus.Engine) *BlockValidator {
	validator := &BlockValidator{
		network: network,
		engine:  engine,
		bc:      blockchain,
	}
	return validator
}

// ValidateBody validates the given block's uncles and verifies the the block
// header's transaction and uncle roots. The headers are assumed to be already
// validated at this point.
func (v *BlockValidator) ValidateBody(block *Block) error {
	// Check whether the block's known, and if not, that it's linkable
	if v.bc.HasBlockAndState(block.Hash(), block.NumberU64()) {
		return ErrKnownBlock
	}
	if !v.bc.HasBlockAndState(block.PreviousBlockHash(), block.NumberU64()-1) {
		if !v.bc.HasBlock(block.PreviousBlockHash(), block.NumberU64()-1) {
			return consensus.ErrUnknownAncestor
		}
		return consensus.ErrPrunedAncestor
	}
	// Header validity is known at this point, check transactions
	header := block.Header()

	if hash := rlpHash(block.LastCommit()); hash != header.LastCommitHash {
		return fmt.Errorf("last commit root hash mismatch: have %x, want %x", hash, header.LastCommitHash)
	}

	if hash := rlpHash(block.ProtocolViolations()); hash != header.ProtocolViolationHash {
		return fmt.Errorf("protocol violation root hash mismatch: have %x, want %x", hash, header.ProtocolViolationHash)
	}

	if hash := types.DeriveSha(block.Transactions()); hash != header.TxHash {
		return fmt.Errorf("transaction root hash mismatch: have %x, want %x", hash, header.TxHash)
	}
	return nil
}

// ValidateState validates the various changes that happen after a state
// transition, the receipt roots and the state root itself.
func (v *BlockValidator) ValidateState(block *Block, statedb *state.StateDB, receipts transaction.Receipts) error {
	// Validate the received block's bloom with the one derived from the generated receipts.
	// For valid blocks this should always validate to true.
	rbloom := types.CreateBloom(receipts)
	if rbloom != header.Bloom {
		return fmt.Errorf("invalid bloom (remote: %x  local: %x)", header.Bloom, rbloom)
	}
	// Tre receipt Trie's root (R = (Tr [[H1, R1], ... [Hn, R1]]))
	receiptSha := types.DeriveSha(receipts)
	if receiptSha != header.ReceiptHash {
		return fmt.Errorf("invalid receipt root hash (remote: %x local: %x)", header.ReceiptHash, receiptSha)
	}
	// Validate the local snapshot against the received snapshot and throw
	// an error if they don't match.
	if snapshot := statedb.IntermediateRoot(true); header.Snapshot != snapshot {
		return fmt.Errorf("invalid merkle root (remote: %x local: %x)", header.Snapshot, snapshot)
	}
	return nil
}

*/
