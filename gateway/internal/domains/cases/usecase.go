package cases

type Usecase interface {
	GetCase(id string) (*Case, error)
	GetAllCases() ([]*Case, error)
}

type usecase struct {
	svc Service
}

func NewUsecase(svc Service) Usecase {
	return &usecase{svc: svc}
}

func (u *usecase) GetCase(id string) (*Case, error) {
	return u.svc.GetCase(id)
}

func (u *usecase) GetAllCases() ([]*Case, error) {
	return u.svc.GetAllCases()
}
