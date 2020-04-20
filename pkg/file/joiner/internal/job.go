package internal

import (
	"context"
	"encoding/binary"
	"io"
	"math"
	"os"

	"github.com/ethersphere/bee/pkg/logging"
	"github.com/ethersphere/bee/pkg/storage"
	"github.com/ethersphere/bee/pkg/swarm"
)

type SimpleJoinerJob struct {
	ctx context.Context
	store storage.Storer
	spanLength int64
	levelCount int
	readCount int64
	cursors [9]int
	data [9][]byte
	dataC chan []byte
	logger logging.Logger
}

func NewSimpleJoinerJob(ctx context.Context, store storage.Storer, rootChunk swarm.Chunk) *SimpleJoinerJob {
	spanLength := binary.LittleEndian.Uint64(rootChunk.Data()[:8])
	levelCount := getLevelsFromLength(int64(spanLength), swarm.SectionSize, swarm.Branches)
	j := &SimpleJoinerJob{
		ctx: ctx,
		store: store,
		spanLength: int64(spanLength),
		levelCount: levelCount,
		dataC: make(chan []byte),
		logger: logging.New(os.Stderr, 5),
	}

	// keeping the data level as 0 index matches the file hasher solution
	j.data[levelCount-1] = rootChunk.Data()[8:]

	go func() {
		err := j.start()
		if err != nil {
			j.logger.Errorf("error in process: %v", err)
			close(j.dataC)
		}
	}()

	return j
}

func (j *SimpleJoinerJob) start() error {
	level := j.levelCount-1 // is first level after root chunk
	for ;j.cursors[level] < len(j.data[level]); {
		cursor := j.cursors[level]
		addressBytes := j.data[level][cursor:cursor+swarm.SectionSize]
		chunkAddress := swarm.NewAddress(addressBytes)
		err := j.descend(level-1, chunkAddress)
		if err != nil {
			return err
		}
		j.cursors[level] += swarm.SectionSize
	}
	return nil
}

func (j *SimpleJoinerJob) descend(level int, address swarm.Address) error {
	j.logger.Debugf("next get: %v", address)
	ch, err := j.store.Get(j.ctx, storage.ModeGetRequest, address)
	if err != nil {
		return err
	}

	if level > 0 {
		if len(j.data[level]) == j.cursors[level]  {
			j.data[level] = ch.Data()[8:]
			j.cursors[level] = 0
		}
		cursor := j.cursors[level]
		nextAddress := swarm.NewAddress(j.data[level][cursor:cursor+swarm.SectionSize])
		err := j.descend(level - 1, nextAddress)
		if err != nil {
			return err
		}
		j.cursors[level] += swarm.SectionSize
	} else {
		data := ch.Data()[8:]
		j.dataC <- data
		j.readCount += int64(len(data))
	}
	return nil
}

func (j *SimpleJoinerJob) Read(b []byte) (n int, err error) {
	select {
	case data, ok := <-j.dataC:
		if !ok {
			j.logger.Debug("eof")
			return 0, io.EOF
		}
		copy(b, data)
		return len(b), nil
	case <-j.ctx.Done():
		return 0, j.ctx.Err()
	}
}

// calculate the last level index which a particular data section count will result in. The returned level will be the level of the root hash
func getLevelsFromLength(l int64, sectionSize int, branches int) int {
	s := int64(sectionSize)
	b := int64(branches)
	if l == 0 {
		return 0
	} else if l <= s*b {
		return 1
	}
	c := (l - 1) / s

	return int(math.Log(float64(c))/math.Log(float64(b)) + 1)
}
