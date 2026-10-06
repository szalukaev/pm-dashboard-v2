package utils

import (
	"encoding/json"
	"net/http"
)

func JSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func Error(w http.ResponseWriter, status int, code string) {
	JSON(w, status, map[string]string{"error": code})
}

func Success(w http.ResponseWriter) {
	JSON(w, http.StatusOK, map[string]bool{"success": true})
}
