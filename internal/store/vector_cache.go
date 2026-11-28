package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"math"
	"os"
	"time"

	"github.com/ewertonls/bigbrain/internal/protocol"
	bolt "go.etcd.io/bbolt"
)

// One bucket per scope, one current entry per memory ID. Only LadybugDB's
// active records can become candidates, even after a crash between stores.
func (s *Store) openVectorCache() error {
	if s.cache != nil {
		return nil
	}
	db, err := bolt.Open(s.cachePath, 0600, &bolt.Options{Timeout: time.Second})
	if err == nil {
		s.cache = db
	}
	return err
}

func (s *Store) removeVector(scope, id string) error {
	if s.cache == nil {
		if _, err := os.Stat(s.cachePath); errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err := s.openVectorCache(); err != nil {
			return err
		}
	}
	return s.cache.Update(func(tx *bolt.Tx) error {
		if bucket := tx.Bucket([]byte(scope)); bucket != nil {
			return bucket.Delete([]byte(id))
		}
		return nil
	})
}

func vectorKey(model *encoder, doc *indexedMemory) [sha256.Size]byte {
	h := sha256.New()
	h.Write(model.fingerprint[:])
	h.Write([]byte(doc.Text))
	return [sha256.Size]byte(h.Sum(nil))
}

// A header with no coordinates records text that has no usable embedding.
// Include a checksum so a damaged coordinate never contributes to scoring.
func packVector(key [sha256.Size]byte, vector []float32) []byte {
	data := make([]byte, sha256.Size, sha256.Size+4+4*len(vector))
	copy(data, key[:])
	for _, n := range vector {
		data = binary.LittleEndian.AppendUint32(data, math.Float32bits(n))
	}
	return binary.LittleEndian.AppendUint32(data, crc32.ChecksumIEEE(data))
}

func unpackVector(data []byte, key [sha256.Size]byte, dim int) ([]float32, bool) {
	const header = sha256.Size + 4
	if len(data) != header && len(data) != header+4*dim {
		return nil, false
	}
	if !bytes.Equal(data[:sha256.Size], key[:]) ||
		crc32.ChecksumIEEE(data[:len(data)-4]) != binary.LittleEndian.Uint32(data[len(data)-4:]) {
		return nil, false
	}
	if len(data) == header {
		return nil, true
	}
	vector := make([]float32, dim)
	for i := range vector {
		vector[i] = math.Float32frombits(binary.LittleEndian.Uint32(data[sha256.Size+i*4:]))
	}
	return vector, true
}

func (s *Store) indexVectors(ctx context.Context, idx *scopeIndex, model *encoder, report func(protocol.IndexProgress)) error {
	if idx.indexedBy == model {
		return nil
	}
	if err := s.openVectorCache(); err != nil {
		return err
	}
	scope := []byte(idx.scope)
	var stale, pending []string
	idx.embeddedDocs = 0
	err := s.cache.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(scope)
		for id, doc := range idx.docs {
			if err := ctx.Err(); err != nil {
				return err
			}
			var data []byte
			if bucket != nil {
				data = bucket.Get([]byte(id))
			}
			var valid bool
			doc.vector, valid = unpackVector(data, vectorKey(model, doc), model.dim)
			if !valid {
				pending = append(pending, id)
				if data != nil {
					stale = append(stale, id)
				}
			} else if len(doc.vector) != 0 {
				idx.embeddedDocs++
			}
		}
		if bucket == nil {
			return nil
		}
		return bucket.ForEach(func(key, _ []byte) error {
			if idx.docs[string(key)] == nil {
				stale = append(stale, string(key))
			}
			return ctx.Err()
		})
	})
	if err != nil {
		return err
	}
	for _, phase := range []struct {
		name string
		ids  []string
	}{{"invalidation", stale}, {"recalculation", pending}} {
		if len(phase.ids) == 0 {
			continue
		}
		update := protocol.IndexProgress{Phase: phase.name, Scope: idx.scope, Total: len(phase.ids)}
		if report != nil {
			report(update)
		}
		for _, id := range phase.ids {
			if err := ctx.Err(); err != nil {
				return err
			}
			err := s.cache.Update(func(tx *bolt.Tx) error {
				bucket, err := tx.CreateBucketIfNotExists(scope)
				if err != nil {
					return err
				}
				if phase.name == "invalidation" {
					return bucket.Delete([]byte(id))
				}
				doc := idx.docs[id]
				doc.vector = model.encode(doc.Text)
				if len(doc.vector) != 0 {
					idx.embeddedDocs++
				}
				return bucket.Put([]byte(id), packVector(vectorKey(model, doc), doc.vector))
			})
			if err != nil {
				return err
			}
			update.Completed++
			if report != nil {
				report(update)
			}
		}
	}
	idx.indexedBy = model
	return nil
}
