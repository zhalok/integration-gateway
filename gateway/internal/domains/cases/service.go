package cases

import "fmt"

type Service interface {
	GetCase(id string) (*Case, error)
	GetAllCases() ([]*Case, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) GetCase(id string) (*Case, error) {
	c, err := s.repo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("get case: %w", err)
	}
	return c, nil
}

func (s *service) GetAllCases() ([]*Case, error) {
	cases, err := s.repo.GetAll()
	if err != nil {
		return nil, fmt.Errorf("get all cases: %w", err)
	}
	return cases, nil
}
