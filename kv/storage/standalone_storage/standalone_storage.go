package standalone_storage

import (
	"github.com/Connor1996/badger"
	"github.com/pingcap-incubator/tinykv/kv/util/engine_util"

	"github.com/pingcap-incubator/tinykv/kv/config"
	"github.com/pingcap-incubator/tinykv/kv/storage"
	"github.com/pingcap-incubator/tinykv/proto/pkg/kvrpcpb"
)

// StandAloneStorage is an implementation of `Storage` for a single-node TinyKV instance. It does not
// communicate with other nodes and all data is stored locally.
type StandAloneStorage struct {
	// Your Data Here (1).
	Kv     *badger.DB
	KvPath string
}

func NewStandAloneStorage(conf *config.Config) *StandAloneStorage {
	// Your Code Here (1).
	// fmt.Println("storage create!")
	return &StandAloneStorage{
		KvPath: conf.DBPath,
	}
}

func (s *StandAloneStorage) Start() error {
	// Your Code Here (1).
	// fmt.Println("storage start!")
	opts := badger.DefaultOptions
	opts.Dir = s.KvPath
	opts.ValueDir = s.KvPath
	db, err := badger.Open(opts)
	s.Kv = db
	if err != nil {
		panic(err)
	}
	return nil
}

func (s *StandAloneStorage) Stop() error {
	// Your Code Here (1).
	// fmt.Println("storage close!")
	s.Kv.Close()
	return nil
}

func (s *StandAloneStorage) Reader(ctx *kvrpcpb.Context) (storage.StorageReader, error) {
	// Your Code Here (1).
	return &StandAloneReader{
		storage: s,
		txn:     s.Kv.NewTransaction(false),
	}, nil
}

func (s *StandAloneStorage) Write(ctx *kvrpcpb.Context, batch []storage.Modify) error {
	// Your Code Here (1).
	for _, m := range batch {
		switch data := m.Data.(type) {
		case storage.Put:
			engine_util.PutCF(s.Kv, data.Cf, data.Key, data.Value)
		case storage.Delete:
			engine_util.DeleteCF(s.Kv, data.Cf, data.Key)
		}
	}
	return nil
}

// StandAloneReader is a StorageReader which reads from a StandAloneStorage.
type StandAloneReader struct {
	storage *StandAloneStorage
	txn     *badger.Txn
}

func (sar *StandAloneReader) GetCF(cf string, key []byte) (val []byte, err error) {
	val, err = engine_util.GetCF(sar.storage.Kv, cf, key)
	if err != nil {
		err = nil
	}
	return
}

func (sar *StandAloneReader) IterCF(cf string) engine_util.DBIterator {
	return engine_util.NewCFIterator(cf, sar.txn)
}

func (sar *StandAloneReader) Close() {
	sar.txn.Discard()
}

// type SasIterator struct {
// 	iter    *engine_util.BadgerIterator
// 	storage *StandAloneStorage
// }

// func NewSasIterator(iter *engine_util.BadgerIterator, storage *StandAloneStorage) *SasIterator {
// 	return &SasIterator{
// 		iter:    iter,
// 		storage: storage,
// 	}
// }

// func (it *SasIterator) Item() engine_util.DBItem {
// 	return it.iter.Item()
// }

// func (it *SasIterator) Valid() bool {
// 	return it.iter.Valid()
// }

// func (it *SasIterator) ValidForPrefix(prefix []byte) bool {
// 	return it.iter.ValidForPrefix(prefix)
// }

// func (it *SasIterator) Close() {
// 	it.iter.Close()
// }

// func (it *SasIterator) Next() {
// 	it.iter.Next()
// }

// func (it *SasIterator) Seek(key []byte) {
// 	it.iter.Seek(key)
// }

// func (it *SasIterator) Rewind() {
// 	it.iter.Rewind()
// }
