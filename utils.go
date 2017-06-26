package main

import (
	"encoding/json"
	"net/http"
)

func writeJSONResponse(rw http.ResponseWriter, code int, result interface{}) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(code)

	json.NewEncoder(rw).Encode(result)
}
