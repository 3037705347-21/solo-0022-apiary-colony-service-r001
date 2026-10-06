package store

import (
	"fmt"

	"example.com/apiary-colony-service/internal/domain"
)

// inspectionLog 承担巡检记录的存储：编号分配、按蜂箱追加与列表读取。
// 类型自身不加锁，并发控制由 Memory 统一负责，调用方须持有对应的锁。
type inspectionLog struct {
	seq   int
	items map[string][]domain.Inspection
}

func newInspectionLog() *inspectionLog {
	return &inspectionLog{items: make(map[string][]domain.Inspection)}
}

// append 分配巡检编号并把记录追加到蜂箱名下，返回带编号的记录。
func (l *inspectionLog) append(hiveID string, inspection domain.Inspection) domain.Inspection {
	l.seq++
	inspection.ID = fmt.Sprintf("inspection-%06d", l.seq)
	inspection.HiveID = hiveID
	l.items[hiveID] = append(l.items[hiveID], inspection)
	return inspection
}

// list 返回蜂箱巡检记录的副本。
func (l *inspectionLog) list(hiveID string) []domain.Inspection {
	items := l.items[hiveID]
	out := make([]domain.Inspection, len(items))
	copy(out, items)
	return out
}

// clone 返回全部巡检记录的深拷贝（含关注项切片），供一致性快照使用。
func (l *inspectionLog) clone() map[string][]domain.Inspection {
	out := make(map[string][]domain.Inspection, len(l.items))
	for id, items := range l.items {
		out[id] = cloneInspections(items)
	}
	return out
}

func cloneInspections(items []domain.Inspection) []domain.Inspection {
	copied := make([]domain.Inspection, len(items))
	copy(copied, items)
	for index := range copied {
		copied[index].Concerns = append([]string{}, items[index].Concerns...)
	}
	return copied
}
