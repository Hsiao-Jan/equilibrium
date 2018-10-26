package types

import "math/big"


/*


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
