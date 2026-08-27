package httpapi

import (
	"mime"
	"net/http"
)

func JSONOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if r.Method != "GET" && mediaType != "application/json" {
			fail(w, ErrJSONContentType{})
			return
		}
		next.ServeHTTP(w, r)
	})
}

type ErrJSONContentType struct{}

func (ErrJSONContentType) Error() string { return "Content-Type 必须为 application/json" }
