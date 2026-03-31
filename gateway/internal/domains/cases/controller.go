package cases

import (
	"encoding/json"
	"net/http"
)

type Controller struct {
	usecase Usecase
}

func NewController(uc Usecase) *Controller {
	return &Controller{usecase: uc}
}

func (c *Controller) GetAllCases(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cases, err := c.usecase.GetAllCases()
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cases)
}
