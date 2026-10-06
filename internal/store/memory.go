package store

import (
	"time"

	"example.com/apiary-colony-service/internal/domain"
)

// Memory 是并发安全的内存仓储，也是唯一的加锁边界和跨角色工作流的编排者。
// 蜂场、蜂箱、巡检、蜂王四类业务事实以及一致性快照分别由独立的存储角色持有，
// Memory 只负责选择读/写锁并在同一临界区内完成跨角色校验与写入。
// 所有写入都在写锁内完成状态校验，读取则通过 Snapshot 得到一致视图。
type Memory struct {
	gate

	apiaries    *apiaryStore
	hives       *hiveStore
	inspections *inspectionStore
	queens      *queenStore
	snapshots   *snapshotStore
}

func NewMemory() *Memory {
	apiaries := newApiaryStore()
	hives := newHiveStore()
	inspections := newInspectionStore()
	queens := newQueenStore()
	return &Memory{
		apiaries:    apiaries,
		hives:       hives,
		inspections: inspections,
		queens:      queens,
		snapshots:   newSnapshotStore(apiaries, hives, inspections, queens),
	}
}

func (m *Memory) CreateApiary(apiary domain.Apiary) error {
	m.Lock()
	defer m.Unlock()
	return m.apiaries.createLocked(apiary)
}

func (m *Memory) GetApiary(id string) (domain.Apiary, bool) {
	m.RLock()
	defer m.RUnlock()
	return m.apiaries.getLocked(id)
}

func (m *Memory) CreateHive(hive domain.Hive) error {
	m.Lock()
	defer m.Unlock()
	// 蜂场归属校验与蜂箱写入在同一把写锁内完成。
	if _, ok := m.apiaries.getLocked(hive.ApiaryID); !ok {
		return domain.NotFound("蜂场", hive.ApiaryID)
	}
	return m.hives.createLocked(hive)
}

func (m *Memory) GetHive(id string) (domain.Hive, bool) {
	m.RLock()
	defer m.RUnlock()
	return m.hives.getLocked(id)
}

// AdvanceHive 在同一把写锁内完成状态读取、迁移校验与写入，避免并发迁移产生交错结果。
func (m *Memory) AdvanceHive(hiveID string, target domain.HiveState, now time.Time) (domain.Transition, error) {
	m.Lock()
	defer m.Unlock()
	return m.hives.advanceLocked(hiveID, target, now)
}

// AddInspection 分配巡检编号、写入记录，并推进蜂箱的最近巡检时间。
// 蜂箱存在与可接受巡检的判定与写入处于同一把锁，避免退役与巡检之间的竞态。
func (m *Memory) AddInspection(hiveID string, inspection domain.Inspection) (domain.Inspection, error) {
	m.Lock()
	defer m.Unlock()
	hive, ok := m.hives.getLocked(hiveID)
	if !ok {
		return domain.Inspection{}, domain.NotFound("蜂箱", hiveID)
	}
	if !hive.AcceptsInspection() {
		return domain.Inspection{}, domain.InvalidState("蜂箱 %q 处于 %q 状态，不接受巡检", hiveID, hive.State)
	}
	saved := m.inspections.appendLocked(hiveID, inspection)

	observedAt := saved.ObservedAt
	hive.LastInspectionAt = &observedAt
	m.hives.putLocked(hive)
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
	m.Lock()
	defer m.Unlock()
	hive, ok := m.hives.getLocked(hiveID)
	if !ok {
		return domain.Queen{}, domain.NotFound("蜂箱", hiveID)
	}
	if !hive.AcceptsQueenChange() {
		return domain.Queen{}, domain.InvalidState("蜂箱 %q 处于 %q 状态，不接受换王", hiveID, hive.State)
	}
	if closeQueenID != "" {
		if err := m.queens.closeLocked(hiveID, closeQueenID, closeState, now); err != nil {
			return domain.Queen{}, err
		}
	}
	saved := m.queens.appendLayingLocked(hiveID, queen)

	hive.ActiveQueenID = saved.ID
	m.hives.putLocked(hive)
	return saved, nil
}

func (m *Memory) ListInspections(hiveID string) ([]domain.Inspection, error) {
	m.RLock()
	defer m.RUnlock()
	// 蜂箱是否存在与记录读取在同一把读锁内判定，保持与写入端一致的错误分类。
	if _, ok := m.hives.getLocked(hiveID); !ok {
		return nil, domain.NotFound("蜂箱", hiveID)
	}
	return m.inspections.listLocked(hiveID), nil
}

func (m *Memory) ListQueens(hiveID string) ([]domain.Queen, error) {
	m.RLock()
	defer m.RUnlock()
	if _, ok := m.hives.getLocked(hiveID); !ok {
		return nil, domain.NotFound("蜂箱", hiveID)
	}
	return m.queens.listLocked(hiveID), nil
}

// Snapshot 返回深拷贝的一致视图，供汇总类读取使用。
// 四个事实存储角色的拷贝都在同一次读锁内完成，快照之间不会混入并发写入。
func (m *Memory) Snapshot() Snapshot {
	m.RLock()
	defer m.RUnlock()
	return m.snapshots.readLocked()
}
