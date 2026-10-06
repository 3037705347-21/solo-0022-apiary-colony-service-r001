package store

import "example.com/apiary-colony-service/internal/domain"

// apiaryStore 是蜂场事实的存储角色：只负责蜂场的录入、按编号读取与快照拷贝。
type apiaryStore struct {
	apiaries map[string]domain.Apiary
}

func newApiaryStore() *apiaryStore {
	return &apiaryStore{apiaries: make(map[string]domain.Apiary)}
}

func (s *apiaryStore) createLocked(apiary domain.Apiary) error {
	if _, ok := s.apiaries[apiary.ID]; ok {
		return domain.Conflict("蜂场 %q 已存在", apiary.ID)
	}
	s.apiaries[apiary.ID] = apiary
	return nil
}

func (s *apiaryStore) getLocked(id string) (domain.Apiary, bool) {
	apiary, ok := s.apiaries[id]
	return apiary, ok
}

// snapshotLocked 返回蜂场表的独立副本。Apiary 不含引用类型字段，直接按值拷贝即可。
func (s *apiaryStore) snapshotLocked() map[string]domain.Apiary {
	out := make(map[string]domain.Apiary, len(s.apiaries))
	for id, apiary := range s.apiaries {
		out[id] = apiary
	}
	return out
}
