package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"
	"github.com/timgluz/wasserspiegel/secret"
)

func BearerAuth(h httprouter.Handle, secretStore secret.Store) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || len(authHeader) < 7 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		authType := strings.ToLower(strings.TrimSpace(authHeader[:7]))
		if authType != "bearer" {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "Unsupported authorization type", http.StatusBadRequest)
			return
		}

		token := strings.TrimSpace(authHeader[7:]) // Extract the token part
		if token == "" {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if secretStore == nil {
			http.Error(w, "Service is not ready", http.StatusInternalServerError)
			return
		}

		if _, err := secretStore.Get(token); err != nil {
			w.Header().Set("WWW-Authenticate", "Bearer")
			if err == secret.ErrSecretNotFound {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			http.Error(w, fmt.Sprintf("Invalid token: %s", err), http.StatusInternalServerError)
			return
		}

		h(w, r, ps)
	}
}
