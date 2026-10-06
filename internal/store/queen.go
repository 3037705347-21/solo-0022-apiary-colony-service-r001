package store

import (
	"fmt"
	"time"

	"example.com/apiary-colony-service/internal/domain"
)

// queenLedger 承担蜂王记录的存储：编号分配、按蜂箱追加、关闭记录与列表读取。
// 类型自身不加锁，并发控制由 Memory 统一负责，调用方须持有对应的锁。
type queenLedger struct {
	seq   int
	items map[string][]domain.Queen
}

func newQueenLedger() *queenLedger {
	return &queenLedger{items: make(map[string][]domain.Queen)}
}

// append 分配蜂王编号并把产卵蜂王记录追加到蜂箱名下，返回带编号的记录。
func (l *queenLedger) append(hiveID string, queen domain.Queen) domain.Queen {
	l.seq++
	queen.ID = fmt.Sprintf("queen-%06d", l.seq)
	queen.HiveID = hiveID
	queen.State = domain.QueenLaying
	l.items[hiveID] = append(l.items[hiveID], queen)
	return queen
}

// close 把蜂箱上的产卵蜂王关闭为给定状态；记录已关闭或缺失时返回错误。
func (l *queenLedger) close(hiveID, queenID string, state domain.QueenState, now time.Time) error {
	items := l.items[hiveID]
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
		l.items[hiveID] = items
		return nil
	}
	return domain.Storage("蜂箱 %q 上的蜂王 %q 记录缺失", hiveID, queenID)
}

// list 返回蜂箱蜂王记录的副本。
func (l *queenLedger) list(hiveID string) []domain.Queen {
	items := l.items[hiveID]
	out := make([]domain.Queen, len(items))
	copy(out, items)
	return out
}

// clone 返回全部蜂王记录的深拷贝（含关闭时间指针），供一致性快照使用。
func (l *queenLedger) clone() map[string][]domain.Queen {
	out := make(map[string][]domain.Queen, len(l.items))
	for id, items := range l.items {
		out[id] = cloneQueens(items)
	}
	return out
}

func cloneQueens(items []domain.Queen) []domain.Queen {
	copied := make([]domain.Queen, len(items))
	copy(copied, items)
	for index := range copied {
		if items[index].ClosedAt != nil {
			closedAt := *items[index].ClosedAt
			copied[index].ClosedAt = &closedAt
		}
	}
	return copied
}
