package api

import (
	"net/http"

	"github.com/tokoonline/app/internal/services/gmc"
)

type GMCHandler struct {
	GMC     *gmc.Service
	BaseURL string
}

func (h *GMCHandler) Feed(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	if err := h.GMC.Write(r.Context(), w, h.BaseURL); err != nil {
		http.Error(w, err.Error(), 500)
	}
}
