package httpapi

import "net/http"

func MethodNotAllowed(w http.ResponseWriter) { w.WriteHeader(http.StatusMethodNotAllowed) }
