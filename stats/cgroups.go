package stats

type CGroupsSampler struct {
}

func NewCGroupsSampler() (*CGroupsSampler, error) {
	return &CGroupsSampler{}, nil
}

func (s *CGroupsSampler) Query() (*ProcMetrics, error) {
	panic("implement me")
}

func (s *CGroupsSampler) Close() error {
	panic("implement me")
}
