package store

import "example.com/apiary-colony-service/internal/domain"

// hiveTable 承担蜂箱档案的存储：编号唯一性校验、按编号读取与整箱更新。
// 类型自身不加锁，并发控制由 Memory 统一负责，调用方须持有对应的锁。
type hiveTable map[string]domain.Hive

func newHiveTable() hiveTable {
	return make(hiveTable)
}

// add 在编号未占用时写入蜂箱，否则返回冲突。
func (t hiveTable) add(hive domain.Hive) error {
	if _, ok := t[hive.ID]; ok {
		return domain.Conflict("蜂箱 %q 已存在", hive.ID)
	}
	t[hive.ID] = hive
	return nil
}

func (t hiveTable) get(id string) (domain.Hive, bool) {
	hive, ok := t[id]
	return hive, ok
}

// put 覆盖写入调用方在同一临界区内修改过的蜂箱记录。
func (t hiveTable) put(hive domain.Hive) {
	t[hive.ID] = hive
}

// clone 返回全部蜂箱的深拷贝（含最近巡检时间指针），供一致性快照使用。
func (t hiveTable) clone() map[string]domain.Hive {
	out := make(map[string]domain.Hive, len(t))
	for id, hive := range t {
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
