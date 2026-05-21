package httpx

import (
	"encoding/json"
	"net/http"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

func JSON(
	w http.ResponseWriter,
	status int,
	data any,
) error {
	w.Header().Set(
		"Content-Type",
		"application/json",
	)
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(data)
}

func Text(
	w http.ResponseWriter,
	status int,
	data string,
) error {
	w.WriteHeader(status)
	_, err := w.Write([]byte(data))
	return err
}

func Error(
	w http.ResponseWriter,
	status int,
	message string,
) error {
	return JSON(
		w,
		status,
		ErrorResponse{Error: message},
	)
}
