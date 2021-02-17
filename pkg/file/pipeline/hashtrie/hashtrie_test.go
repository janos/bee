package hashtrie_test

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/ethersphere/bee/pkg/file/pipeline"
	"github.com/ethersphere/bee/pkg/file/pipeline/bmt"
	"github.com/ethersphere/bee/pkg/file/pipeline/hashtrie"
	"github.com/ethersphere/bee/pkg/file/pipeline/store"
	"github.com/ethersphere/bee/pkg/storage"
	"github.com/ethersphere/bee/pkg/storage/mock"
	"github.com/ethersphere/bee/pkg/swarm"
)

var (
	addr swarm.Address
	span []byte
	ctx  = context.Background()
	mode = storage.ModePutUpload
)

func init() {
	b := make([]byte, 32)
	b[31] = 0x01
	addr = swarm.NewAddress(b)

	span = make([]byte, 8)
	binary.LittleEndian.PutUint64(span, 1)
}

func TestLevels(t *testing.T) {
	var (
		branching = 4
		chunkSize = 128
		hashSize  = 32
	)

	// to create a level wrap we need to do branching^(level-1) writes
	for _, tc := range []struct {
		desc   string
		writes int
	}{
		{
			desc:   "2 at L1",
			writes: 2,
		},
		{
			desc:   "1 at L2, 1 at L1", // dangling chunk
			writes: 16 + 1,
		},
		{
			desc:   "1 at L3, 1 at L2, 1 at L1",
			writes: 64 + 16 + 1,
		},
		{
			desc:   "1 at L3, 2 at L2, 1 at L1",
			writes: 64 + 16 + 16 + 1,
		},
		{
			desc:   "1 at L5, 1 at L1",
			writes: 1024 + 1,
		},
		{
			desc:   "1 at L5, 1 at L3",
			writes: 1024 + 1,
		},
		{
			desc:   "2 at L5, 1 at L1",
			writes: 1024 + 1024 + 1,
		},
		{
			desc:   "3 at L5, 2 at L3, 1 at L1",
			writes: 1024 + 1024 + 1024 + 64 + 64 + 1,
		},
		{
			desc:   "1 at L7, 1 at L1",
			writes: 4096 + 1,
		},
		{
			desc:   "1 at L8", // balanced trie - all good
			writes: 16384,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			s := mock.NewStorer()
			pf := func() pipeline.ChainWriter {
				lsw := store.NewStoreWriter(ctx, s, mode, nil)
				return bmt.NewBmtWriter(lsw)
			}

			ht := hashtrie.NewHashTrieWriter(chunkSize, branching, hashSize, pf)

			for i := 0; i < tc.writes; i++ {
				a := &pipeline.PipeWriteArgs{Ref: addr.Bytes(), Span: span}
				err := ht.ChainWrite(a)
				if err != nil {
					t.Fatal(err)
				}
			}

			ref, err := ht.Sum()
			if err != nil {
				t.Fatal(err)
			}

			rootch, err := s.Get(ctx, storage.ModeGetRequest, swarm.NewAddress(ref))
			if err != nil {
				t.Fatal(err)
			}

			//check the span. since write spans are 1 value 1, then expected span == tc.writes
			sp := binary.LittleEndian.Uint64(rootch.Data()[:swarm.SpanSize])
			if sp != uint64(tc.writes) {
				t.Fatalf("want span %d got %d", tc.writes, sp)
			}
		})
	}
}

func TestLevels_TrieFull(t *testing.T) {
	var (
		branching = 4
		chunkSize = 128
		hashSize  = 32
		writes    = 16384 // this is to get a balanced trie
		s         = mock.NewStorer()
		pf        = func() pipeline.ChainWriter {
			lsw := store.NewStoreWriter(ctx, s, mode, nil)
			return bmt.NewBmtWriter(lsw)
		}

		ht = hashtrie.NewHashTrieWriter(chunkSize, branching, hashSize, pf)
	)

	// to create a level wrap we need to do branching^(level-1) writes
	for i := 0; i < writes; i++ {
		a := &pipeline.PipeWriteArgs{Ref: addr.Bytes(), Span: span}
		err := ht.ChainWrite(a)
		if err != nil {
			t.Fatal(err)
		}
	}

	a := &pipeline.PipeWriteArgs{Ref: addr.Bytes(), Span: span}
	err := ht.ChainWrite(a)
	if !errors.Is(err, hashtrie.ErrTrieFull) {
		t.Fatal(err)
	}

	// it is questionable whether the writer should go into some
	// corrupt state after the last write which causes the trie full
	// error, in which case we would return an error on Sum()
	_, err = ht.Sum()
	if err != nil {
		t.Fatal(err)
	}
}

func TestOneLevel(t *testing.T) {
	s := mock.NewStorer()
	ctx := context.Background()
	mode := storage.ModePutUpload
	pf := func() pipeline.ChainWriter {
		lsw := store.NewStoreWriter(ctx, s, mode, nil)
		return bmt.NewBmtWriter(lsw)
	}

	ht := hashtrie.NewHashTrieWriter(64, 2, 32, pf)
	spb := make([]byte, 8)
	binary.LittleEndian.PutUint64(spb, 1)

	for i := 0; i < 2; i++ {
		a := &pipeline.PipeWriteArgs{Ref: addr.Bytes(), Span: spb}
		ht.ChainWrite(a)
	}

	ref, err := ht.Sum()
	if err != nil {
		t.Fatal(err)
	}
	exphash := "281907c4199cd2b05b80469d2af5be103cc1317dcf78d5d9b95694aadb2d4994"
	if a := hex.EncodeToString(ref); a != exphash {
		t.Fatalf("expected hash %s but got %s", exphash, a)
	}
}
