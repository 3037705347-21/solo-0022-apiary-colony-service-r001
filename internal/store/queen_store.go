package store

import (
	"fmt"
	"time"

	"example.com/apiary-colony-service/internal/domain"
)

// queenStore 是蜂王事实的存储角色：负责蜂王编号分配、产卵王写入、旧王关闭
// 以及快照拷贝。蜂箱是否接受换王由 Memory 门面在同一临界区内判定。
type queenStore struct {
	items map[string][]domain.Queen
	seq   int
}

func newQueenStore() *queenStore {
	// 当前没有蜂王的蜂箱在读取时返回 nil，保持与 map 缺省值一致。
	return &queenStore{items: make(map[string][]domain.Queen)}
}

// closeLocked 在调用方已持写锁的前提下把指定产卵蜂王置为关闭状态。
// 关闭后不可再改；记录缺失属于存储内部不一致。
func (s *queenStore) closeLocked(hiveID, queenID string, state domain.QueenState, now time.Time) error {
	items := s.items[hiveID]
	for index, item := range items {
		if item.ID != queenID {
			continue
		}
		if !item.IsLaying() {
			return domain.Conflict("蜂王 %q 已经关闭", queenID)
		}
		closedAt := now.UTC()
		item.State = state
		item.ClosedAt = &closedAt
		items[index] = item
		s.items[hiveID] = items
		return nil
	}
	return domain.Storage("蜂箱 %q 上的蜂王 %q 记录缺失", hiveID, queenID)
}

// appendLayingLocked 分配蜂王编号、强制为产卵状态并追加记录，编号在同一临界区顺序推进。
func (s *queenStore) appendLayingLocked(hiveID string, queen domain.Queen) domain.Queen {
	s.seq++
	queen.ID = fmt.Sprintf("queen-%06d", s.seq)
	queen.HiveID = hiveID
	queen.State = domain.QueenLaying
	s.items[hiveID] = append(s.items[hiveID], queen)
	return queen
}

// listLocked 返回某只蜂箱蜂王记录的独立副本，供单次读取使用。
func (s *queenStore) listLocked(hiveID string) []domain.Queen {
	items := s.items[hiveID]
	out := make([]domain.Queen, len(items))
	copy(out, items)
	return out
}

// snapshotLocked 返回全部蜂王记录的深拷贝，关闭时间指针必须与快照外的数据隔离。
func (s *queenStore) snapshotLocked() map[string][]domain.Queen {
	out := make(map[string][]domain.Queen, len(s.items))
	for id, items := range s.items {
		copied := make([]domain.Queen, len(items))
		copy(copied, items)
		for index := range copied {
			if items[index].ClosedAt != nil {
				closedAt := *items[index].ClosedAt
				copied[index].ClosedAt = &closedAt
			}
		}
		out[id] = copied
	}
	return out
}
