package store

import "example.com/apiary-colony-service/internal/domain"

// apiaryTable 承担蜂场档案的存储：编号唯一性校验与按编号读取。
// 类型自身不加锁，并发控制由 Memory 统一负责，调用方须持有对应的锁。
type apiaryTable map[string]domain.Apiary

func newApiaryTable() apiaryTable {
	return make(apiaryTable)
}

// add 在编号未占用时写入蜂场，否则返回冲突。
func (t apiaryTable) add(apiary domain.Apiary) error {
	if _, ok := t[apiary.ID]; ok {
		return domain.Conflict("蜂场 %q 已存在", apiary.ID)
	}
	t[apiary.ID] = apiary
	return nil
}

func (t apiaryTable) get(id string) (domain.Apiary, bool) {
	apiary, ok := t[id]
	return apiary, ok
}

func (t apiaryTable) has(id string) bool {
	_, ok := t[id]
	return ok
}

// clone 返回全部蜂场的拷贝，供一致性快照使用。
func (t apiaryTable) clone() map[string]domain.Apiary {
	out := make(map[string]domain.Apiary, len(t))
	for id, apiary := range t {
		out[id] = apiary
	}
	return out
}
