package store

import (
	"fmt"

	"example.com/apiary-colony-service/internal/domain"
)

// inspectionStore 是巡检事实的存储角色：负责巡检编号分配、追加写入、
// 按蜂箱列出与快照拷贝。它不关心蜂箱状态，可接受性由 Memory 门面在同一临界区内判定。
type inspectionStore struct {
	items map[string][]domain.Inspection
	seq   int
}

func newInspectionStore() *inspectionStore {
	return &inspectionStore{items: make(map[string][]domain.Inspection)}
}

// appendLocked 分配巡检编号并追加记录，编号在同一临界区顺序推进。
func (s *inspectionStore) appendLocked(hiveID string, inspection domain.Inspection) domain.Inspection {
	s.seq++
	inspection.ID = fmt.Sprintf("inspection-%06d", s.seq)
	inspection.HiveID = hiveID
	s.items[hiveID] = append(s.items[hiveID], inspection)
	return inspection
}

// listLocked 返回某只蜂箱巡检记录的独立副本，供单次读取使用。
func (s *inspectionStore) listLocked(hiveID string) []domain.Inspection {
	items := s.items[hiveID]
	out := make([]domain.Inspection, len(items))
	copy(out, items)
	return out
}

// snapshotLocked 返回全部巡检记录的深拷贝，关注项切片必须与快照外的数据隔离。
func (s *inspectionStore) snapshotLocked() map[string][]domain.Inspection {
	out := make(map[string][]domain.Inspection, len(s.items))
	for id, items := range s.items {
		copied := make([]domain.Inspection, len(items))
		copy(copied, items)
		for index := range copied {
			copied[index].Concerns = append([]string{}, items[index].Concerns...)
		}
		out[id] = copied
	}
	return out
}
