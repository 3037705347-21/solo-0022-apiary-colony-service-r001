package store

import (
	"time"

	"example.com/apiary-colony-service/internal/domain"
)

// hiveStore 是蜂箱事实的存储角色：负责蜂箱的录入、按编号读取、状态迁移
// 以及由跨角色工作流（巡检、换王）回写的蜂箱派生字段。
type hiveStore struct {
	hives map[string]domain.Hive
}

func newHiveStore() *hiveStore {
	return &hiveStore{hives: make(map[string]domain.Hive)}
}

func (s *hiveStore) createLocked(hive domain.Hive) error {
	if _, ok := s.hives[hive.ID]; ok {
		return domain.Conflict("蜂箱 %q 已存在", hive.ID)
	}
	s.hives[hive.ID] = hive
	return nil
}

func (s *hiveStore) getLocked(id string) (domain.Hive, bool) {
	hive, ok := s.hives[id]
	return hive, ok
}

// putLocked 回写整只蜂箱，供同一临界区内的巡检、换王工作流更新派生字段。
func (s *hiveStore) putLocked(hive domain.Hive) {
	s.hives[hive.ID] = hive
}

// advanceLocked 在调用方已持写锁的前提下完成迁移校验与状态写入。
func (s *hiveStore) advanceLocked(hiveID string, target domain.HiveState, _ time.Time) (domain.Transition, error) {
	hive, ok := s.hives[hiveID]
	if !ok {
		return domain.Transition{}, domain.NotFound("蜂箱", hiveID)
	}
	if hive.State == target {
		return domain.Transition{}, domain.InvalidState("蜂箱 %q 已经处于状态 %q", hiveID, target)
	}
	if !domain.CanTransitionHive(hive.State, target) {
		return domain.Transition{}, domain.InvalidState(
			"蜂箱 %q 不允许从 %q 迁移到 %q", hiveID, hive.State, target)
	}
	from := hive.State
	hive.State = target
	s.hives[hiveID] = hive
	return domain.Transition{HiveID: hiveID, From: from, To: target}, nil
}

// snapshotLocked 返回蜂箱表的深拷贝，最近巡检时间指针必须与快照外的数据隔离。
func (s *hiveStore) snapshotLocked() map[string]domain.Hive {
	out := make(map[string]domain.Hive, len(s.hives))
	for id, hive := range s.hives {
		out[id] = cloneHive(hive)
	}
	return out
}

func cloneHive(hive domain.Hive) domain.Hive {
	if hive.LastInspectionAt != nil {
		observedAt := *hive.LastInspectionAt
		hive.LastInspectionAt = &observedAt
	}
	return hive
}
