// Package godorigin defines an optional, exact Chrome test-wallet boundary.
// An extension ID is public metadata, not client authentication or wallet ownership.
package godorigin

import (
	"net/http"
	"regexp"
	"strings"
)

var extension = regexp.MustCompile(`^chrome-extension://[a-p]{32}$`)

func ValidExtension(origin string) bool { return origin == "" || extension.MatchString(origin) }

func Allowed(website, extensionOrigin, origin string) bool {
	return origin != "" && (origin == website || extensionOrigin != "" && origin == extensionOrigin)
}

// CORS is called only after exact origin, proxy peer, host and path validation.
// It never allows credentials, arbitrary request headers or other HTTP methods.
func CORS(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Vary", "Origin")
	if r.Method != http.MethodOptions {
		return false
	}
	valid := len(r.Header.Values("Access-Control-Request-Method")) == 1 && r.Header.Get("Access-Control-Request-Method") == "POST" && len(r.Header.Values("Access-Control-Request-Headers")) <= 1
	for _, value := range strings.Split(r.Header.Get("Access-Control-Request-Headers"), ",") {
		if value != "" && !strings.EqualFold(strings.TrimSpace(value), "Content-Type") {
			valid = false
		}
	}
	if !valid {
		http.Error(w, "preflight rejected", http.StatusForbidden)
		return true
	}
	w.WriteHeader(http.StatusNoContent)
	return true
}
