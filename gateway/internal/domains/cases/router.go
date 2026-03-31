package cases

import "net/http"

func (c *Controller) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/cases", c.GetAllCases)
}
