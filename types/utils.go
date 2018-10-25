package types

import "math/big"

// DeriveChainID derives the chain id from the given v parameter.
func DeriveChainID(v *big.Int) *big.Int {
	if v.BitLen() <= 64 {
		v := v.Uint64()
		if v == 27 || v == 28 {
			return new(big.Int)
		}
		return new(big.Int).SetUint64((v - 35) / 2)
	}
	v = new(big.Int).Sub(v, big.NewInt(35))
	return v.Div(v, big.NewInt(2))
}

/*
func rlpHash(x interface{}) (h Hash) {
	hw := sha3.NewKeccak256()
	rlp.Encode(hw, x)
	hw.Sum(h[:0])
	return h
}

// SignTx signs the transaction using the given signer and private key
func SignTx(tx *Transaction, signer Signer, prv *ecdsa.PrivateKey) (*types.Transaction, error) {
	h := signer.Hash(tx)
	sig, err := crypto.Sign(h.Bytes(), prv)
	if err != nil {
		return nil, err
	}
	return tx.WithSignature(signer, sig)
}

// SignProposal signs the proposal using the given signer and private key
func SignProposal(proposal *Proposal, signer Signer, prv *ecdsa.PrivateKey) (*miningtypes.Proposal, error) {
	h := signer.Hash(proposal)
	sig, err := crypto.Sign(h[:], prv)
	if err != nil {
		return nil, err
	}
	return proposal.WithSignature(signer, sig)

}

// SignVote signs the vote using the given signer and private key
func SignVote(vote *Vote, signer Signer, prv *ecdsa.PrivateKey) (*miningtypes.Vote, error) {
	h := signer.Hash(vote)
	sig, err := crypto.Sign(h[:], prv)
	if err != nil {
		return nil, err
	}
	return vote.WithSignature(signer, sig)
}

*/
