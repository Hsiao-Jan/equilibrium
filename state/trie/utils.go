package trie

import (
	"bytes"

	"github.com/kowala-tech/equilibrium/crypto"
	"github.com/kowala-tech/equilibrium/encoding/rlp"
)

type DerivableList interface {
	Len() int
	GetRlp(i int) []byte
}

func DeriveSha(list DerivableList) crypto.Hash {
	keybuf := new(bytes.Buffer)
	trie := new(Trie)
	for i := 0; i < list.Len(); i++ {
		keybuf.Reset()
		rlp.Encode(keybuf, uint(i))
		trie.Update(keybuf.Bytes(), list.GetRlp(i))
	}
	return trie.Hash()
}
