package health

import (
	"encoding/json"
	"github.com/rs/zerolog/log"
	"net/http"
)

// Status is the health status of the service
type Status string

const (
	// Ok status means that the service is operating nominally
	Ok = Status("OK")

	// Error status means that something is going wrong with the service
	Error = Status("Error")
)

// ServiceStatus holds information about the health of the bqmetricsd service
type ServiceStatus struct {
	Status Status `json:"status"`
}

// Handler will handle HTTP requests to the health endpoint
func (hs ServiceStatus) Handler(w http.ResponseWriter, _ *http.Request) {
	data, err := json.Marshal(hs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	switch hs.Status {
	case Ok:
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusInternalServerError)
	}

	if _, err = w.Write(data); err != nil {
		log.Err(err).Msg("error when writing health http response")
	}
}
