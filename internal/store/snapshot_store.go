package store

// snapshotStore 是只读的一致性视图角色：它本身不持有数据，也不加锁，
// 只负责在 Memory 已持有读锁的前提下，把四个事实存储角色的深拷贝
// 组装成一份 Snapshot，保证快照是一次读取的一致视图。
type snapshotStore struct {
	apiaries    *apiaryStore
	hives       *hiveStore
	inspections *inspectionStore
	queens      *queenStore
}

func newSnapshotStore(apiaries *apiaryStore, hives *hiveStore, inspections *inspectionStore, queens *queenStore) *snapshotStore {
	return &snapshotStore{
		apiaries:    apiaries,
		hives:       hives,
		inspections: inspections,
		queens:      queens,
	}
}

func (s *snapshotStore) readLocked() Snapshot {
	return Snapshot{
		Apiaries: s.apiaries.snapshotLocked(),
		// 蜂箱、巡检、蜂王的拷贝循环交错执行，但全程都在同一次 RLock 内，
		// 因此不会看到任何写者的中间状态。
		Hives:             s.hives.snapshotLocked(),
		InspectionsByHive: s.inspections.snapshotLocked(),
		QueensByHive:      s.queens.snapshotLocked(),
	}
}
