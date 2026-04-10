package middleware

import (
	"net/http"
	"runtime/debug"
)

type recoveryLogger interface {
	Printf(format string, args ...interface{})
}

// Recovery catches panics in request handlers, logs stack, and returns 500.
func Recovery(logger recoveryLogger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				if logger != nil {
					logger.Printf("panic recovered: %v method=%s path=%s\n%s", rec, r.Method, r.URL.Path, debug.Stack())
				}
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// RecoveryFunc is a helper for wrapping a handler function.
func RecoveryFunc(logger recoveryLogger, next http.HandlerFunc) http.Handler {
	return Recovery(logger, next)
}
