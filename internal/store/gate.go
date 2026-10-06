package store

import "sync"

// gate 是所有存储角色共享的唯一读写门闩。
// 各角色自身不加锁，只暴露 *Locked 方法，由 Memory 门面在进入临界区后调用，
// 这样跨角色的状态校验与写入仍处在同一临界区内。
type gate struct {
	sync.RWMutex
}
