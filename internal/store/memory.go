package store

import (
	"sync"
	"time"

	"example.com/apiary-colony-service/internal/domain"
)

// Memory 是并发安全的内存仓储。它持有唯一的锁，并把各类业务事实委托给
// 各自的存储角色：写入方法在写锁内完成跨角色校验与修改，处于同一临界区；
// 读取则通过 Snapshot 在一次读锁内得到一致视图。
type Memory struct {
	mu sync.RWMutex

	apiaries    apiaryTable
	hives       hiveTable
	inspections *inspectionLog
	queens      *queenLedger
}

func NewMemory() *Memory {
	return &Memory{
		apiaries:    newApiaryTable(),
		hives:       newHiveTable(),
		inspections: newInspectionLog(),
		queens:      newQueenLedger(),
	}
}

func (m *Memory) CreateApiary(apiary domain.Apiary) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.apiaries.add(apiary)
}

func (m *Memory) GetApiary(id string) (domain.Apiary, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.apiaries.get(id)
}

// CreateHive 在写锁内确认蜂场存在并校验蜂箱编号唯一，再写入蜂箱。
func (m *Memory) CreateHive(hive domain.Hive) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.apiaries.has(hive.ApiaryID) {
		return domain.NotFound("蜂场", hive.ApiaryID)
	}
	return m.hives.add(hive)
}

func (m *Memory) GetHive(id string) (domain.Hive, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.hives.get(id)
}

// AdvanceHive 在同一把写锁内完成状态读取、迁移校验与写入，避免并发迁移产生交错结果。
func (m *Memory) AdvanceHive(hiveID string, target domain.HiveState, _ time.Time) (domain.Transition, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	hive, ok := m.hives.get(hiveID)
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
	m.hives.put(hive)
	return domain.Transition{HiveID: hiveID, From: from, To: target}, nil
}

// AddInspection 分配巡检编号、写入记录，并推进蜂箱的最近巡检时间。
// 蜂箱存在与可接受巡检的判定与写入处于同一把锁，避免退役与巡检之间的竞态。
func (m *Memory) AddInspection(hiveID string, inspection domain.Inspection) (domain.Inspection, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	hive, ok := m.hives.get(hiveID)
	if !ok {
		return domain.Inspection{}, domain.NotFound("蜂箱", hiveID)
	}
	if !hive.AcceptsInspection() {
		return domain.Inspection{}, domain.InvalidState("蜂箱 %q 处于 %q 状态，不接受巡检", hiveID, hive.State)
	}
	saved := m.inspections.append(hiveID, inspection)

	observedAt := saved.ObservedAt
	hive.LastInspectionAt = &observedAt
	m.hives.put(hive)
	return saved, nil
}

// AddQueen 关闭原有产卵蜂王并写入新蜂王，两件事在同一把锁内完成，
// 保证任意时刻同一蜂箱至多只有一只 laying 蜂王。
func (m *Memory) AddQueen(
	hiveID string,
	queen domain.Queen,
	closeQueenID string,
	closeState domain.QueenState,
	now time.Time,
) (domain.Queen, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	hive, ok := m.hives.get(hiveID)
	if !ok {
		return domain.Queen{}, domain.NotFound("蜂箱", hiveID)
	}
	if !hive.AcceptsQueenChange() {
		return domain.Queen{}, domain.InvalidState("蜂箱 %q 处于 %q 状态，不接受换王", hiveID, hive.State)
	}
	if closeQueenID != "" {
		if err := m.queens.close(hiveID, closeQueenID, closeState, now); err != nil {
			return domain.Queen{}, err
		}
	}
	saved := m.queens.append(hiveID, queen)

	hive.ActiveQueenID = saved.ID
	m.hives.put(hive)
	return saved, nil
}

func (m *Memory) ListInspections(hiveID string) ([]domain.Inspection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.hives.get(hiveID); !ok {
		return nil, domain.NotFound("蜂箱", hiveID)
	}
	return m.inspections.list(hiveID), nil
}

func (m *Memory) ListQueens(hiveID string) ([]domain.Queen, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.hives.get(hiveID); !ok {
		return nil, domain.NotFound("蜂箱", hiveID)
	}
	return m.queens.list(hiveID), nil
}

// Snapshot 返回深拷贝的一致视图，供汇总类读取使用。
func (m *Memory) Snapshot() Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return Snapshot{
		Apiaries:          m.apiaries.clone(),
		Hives:             m.hives.clone(),
		InspectionsByHive: m.inspections.clone(),
		QueensByHive:      m.queens.clone(),
	}
}
