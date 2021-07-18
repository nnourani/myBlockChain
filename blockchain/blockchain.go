package blockchain

import (
	"fmt"
	"os"
	"runtime"

	"github.com/dgraph-io/badger"
)

const (
	dbPath = "./tmp/blocks"

	// This can be used to verify that the blockchain exists
	dbFile = "./tmp/blocks/MANIFEST"

	// This is arbitrary data for our genesis block
	genesisData = "First Transaction from Genesis"
)

type BlockChain struct {
	LastHash []byte
	Database *badger.DB
}

type BlockChainIterator struct {
	CurrentHash []byte
	Database    *badger.DB
}

func DBexists(db string) bool {
	if _, err := os.Stat(db); os.IsNotExist(err) {
		return false
	}

	return true
}
func InitBlockChain(address string) *BlockChain {
	var lastHash []byte

	if DBexists(dbFile) {
		fmt.Println("blockchain already exists")
		runtime.Goexit()
	}

	opts := badger.DefaultOptions(dbPath)
	db, err := badger.Open(opts)
	Handle(err)

	err = db.Update(func(txn *badger.Txn) error {
		cbtx := CoinBaseTx(address, genesisData)
		genesis := Genesis(cbtx)
		fmt.Println("Genesis proved")
		err = txn.Set(genesis.Hash, genesis.Serialize())
		Handle(err)
		err = txn.Set([]byte("lh"), genesis.Hash)

		lastHash = genesis.Hash

		return err

	})

	Handle(err)

	blockchain := BlockChain{lastHash, db}
	return &blockchain
}

//I Know we don't reference address anywhere in here. Keep it anyway.
func ContinueBlockChain(address string) *BlockChain {
	if DBexists(dbFile) == false {
		fmt.Println("No blockchain found, please create one first")
		runtime.Goexit()
	}

	var lastHash []byte

	opts := badger.DefaultOptions(dbPath)
	db, err := badger.Open(opts)
	Handle(err)

	err = db.Update(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("lh"))
		Handle(err)
		err = item.Value(func(val []byte) error {
			lastHash = val
			return nil
		})
		Handle(err)
		return err
	})
	Handle(err)

	chain := BlockChain{lastHash, db}
	return &chain
}

func (chain *BlockChain) FindUnspentTransactions(address string) []Transaction {
    var unspentTxs []Transaction

    spentTXNs := make(map[string][]int)

    iter := chain.Iterator()

    for {
        block := iter.Next()

        for _, tx := range block.Transactions {
            txID := hex.EncodeToString(tx.ID)

        Outputs:
            for outIdx, out := range tx.Outputs {
                if spentTXNs[txID] != nil {
                    for _, spentOut := range spentTXNs[txID] {
                        if spentOut == outIdx {
                            continue Outputs
                        }
                    }
                }
                if out.CanBeUnlocked(address){
                    unspentTxs = append(unspentTxs, *tx)
                }
            }
            if tx.IsCoinbase() == false {
                for _, in := range tx.Inputs {
                    if in.CanUnlock(address) {
                        inTxID := hex.EncodeToString(in.ID)
                        spentTXNs[inTxID] = append(spentTXNs[inTxID], in.Out)
                    }
            }
        }
        if len(block.PrevHash) == 0 {
            break
        }
    }
    return unspentTxs

}



func (chain *BlockChain) AddBlock(transactions []*Transaction) {
    var lastHash []byte

    err := chain.Database.View(func(txn *badger.Txn) error {
        item, err := txn.Get([]byte("lh"))
        Handle(err)
        err = item.Value(func(val []byte) error {
            lastHash = val
            return nil
        })
        Handle(err)
        return err
    })
    Handle(err)

    newBlock := CreateBlock(transactions, lastHash) //THIS LINE CHANGED
    err = chain.Database.Update(func(transaction *badger.Txn) error {
        err := transaction.Set(newBlock.Hash, newBlock.Serialize())
        Handle(err)
        err = transaction.Set([]byte("lh"), newBlock.Hash)

        chain.LastHash = newBlock.Hash
        return err
    })
    Handle(err)
}

func (chain *BlockChain) Iterator() *BlockChainIterator {
	iterator := BlockChainIterator{chain.LastHash, chain.Database}

	return &iterator
}

func (iterator *BlockChainIterator) Next() *Block {
	var block *Block

	err := iterator.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get(iterator.CurrentHash)
		Handle(err)

		err = item.Value(func(val []byte) error {
			block = Deserialize(val)
			return nil
		})
		Handle(err)
		return err
	})
	Handle(err)

	iterator.CurrentHash = block.PrevHash

	return block
}
