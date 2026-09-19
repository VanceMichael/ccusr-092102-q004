package evidence

// SeedDemo 在存储为空时写入一组演示数据：十国制度背景、若干课程版本、
// 已获批/待审/被修订的映射、一条政策建议和一个绑定口径的试点。
func (s *Service) SeedDemo() error {
	if s.storeHasData() {
		return nil
	}
	return s.seed()
}
