package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"net/http"
)

func writeJSONResponse(rw http.ResponseWriter, code int, result interface{}) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(code)

	json.NewEncoder(rw).Encode(result)
}

func getHashPath(buff []byte) string {
	hasher := md5.New()
	hasher.Write(buff)
	hash := hex.EncodeToString(hasher.Sum(nil))
	hashPathPart := hash[0:2] + "/" + hash[2:4] + "/"

	return hashPathPart
}
