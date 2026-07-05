package api

import (
	"errors"
	"net/http"
)

const (
	generateErrEmptyGroup         = "empty_group"
	generateErrInvalidFinal       = "invalid_final"
	generateErrUnknownOutboundRef = "unknown_outbound_ref"
	generateErrGroupCycle         = "group_cycle"
	generateErrAutoCountryEmpty   = "auto_country_empty"
	generateErrChainProxyEmpty    = "chain_proxy_empty"
)

type generationErrorDetails struct {
	Kind        string   `json:"kind"`
	Panel       string   `json:"panel,omitempty"`
	GroupTag    string   `json:"groupTag,omitempty"`
	OutboundTag string   `json:"outboundTag,omitempty"`
	Cycle       []string `json:"cycle,omitempty"`
}

type generationError struct {
	message string
	details generationErrorDetails
}

func (e *generationError) Error() string {
	return e.message
}

func newGenerationError(message string, details generationErrorDetails) error {
	return &generationError{message: message, details: details}
}

func respondGenerationError(w http.ResponseWriter, status int, code string, err error) {
	var ge *generationError
	if errors.As(err, &ge) {
		respondErrorWithDetails(w, status, code, ge.Error(), ge.details)
		return
	}
	respondError(w, status, code, err.Error())
}

func respondBadGenerationRequest(w http.ResponseWriter, err error) {
	respondGenerationError(w, http.StatusBadRequest, "bad_request", err)
}

func respondGenerateFailure(w http.ResponseWriter, err error) {
	respondGenerationError(w, http.StatusUnprocessableEntity, "generate_error", err)
}
