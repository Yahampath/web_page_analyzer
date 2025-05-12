package handlers

import "net/http"


type ReadyHandler struct{
	Metrics struct{}
}

func NewReadyHandler() *ReadyHandler {
	return &ReadyHandler{
		Metrics: struct{}{},
	}
}

func (h *ReadyHandler) Handle(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

