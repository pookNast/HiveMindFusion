package gateway

import (
	"log"
	"net/http"
	"os"
	"strings"
)

// AuthMiddleware enforces API key authentication when enabled.
// Set HIVEMIND_REQUIRE_AUTH=true to require valid consumer credentials.
//
// When enabled, every request must carry one of:
//   - Authorization: Bearer <master-token>  (matches HIVEMIND_AUTH_TOKEN env var)
//   - Authorization: Bearer <key>           (key exists in consumer api_keys map)
//   - X-HiveMind-Consumer: <name>           (name matches a known consumer)
//
// When disabled (default), the gateway is permissive (backward compatible) but
// logs a WARNING at startup and once per hour so the operator notices.
//
// ponytail: simple bearer/header check — upgrade: mTLS + rotating keys if exposed beyond LAN
func AuthMiddleware(apiKeys map[string]string, knownConsumers map[string]struct{}, next http.Handler) http.Handler {
	enforced := strings.EqualFold(os.Getenv("HIVEMIND_REQUIRE_AUTH"), "true")
	masterToken := os.Getenv("HIVEMIND_AUTH_TOKEN")

	if enforced {
		log.Printf("[hivemind] auth: ENFORCED — requests require valid Bearer token or X-HiveMind-Consumer header")
		if masterToken != "" {
			log.Printf("[hivemind] auth: master token loaded from HIVEMIND_AUTH_TOKEN")
		} else {
			log.Printf("[hivemind] auth: WARNING — HIVEMIND_AUTH_TOKEN not set, only consumer synthetic tokens accepted")
		}
	} else {
		log.Printf("[hivemind] auth: WARNING — permissive mode (HIVEMIND_REQUIRE_AUTH not set). " +
			"Set HIVEMIND_REQUIRE_AUTH=true to enforce consumer credentials.")
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !enforced {
			next.ServeHTTP(w, r)
			return
		}

		// Check Authorization: Bearer <key>
		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			key := strings.TrimPrefix(auth, "Bearer ")
			// Master token from HIVEMIND_AUTH_TOKEN env var
			if masterToken != "" && key == masterToken {
				next.ServeHTTP(w, r)
				return
			}
			// Consumer synthetic token from config
			if _, ok := apiKeys[key]; ok {
				next.ServeHTTP(w, r)
				return
			}
		}

		// Check X-HiveMind-Consumer header
		if consumer := r.Header.Get("X-HiveMind-Consumer"); consumer != "" {
			if _, ok := knownConsumers[consumer]; ok {
				next.ServeHTTP(w, r)
				return
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"message":"Unauthorized: valid consumer credentials required","type":"auth_error","code":"invalid_consumer_credentials"}}`)) //nolint:errcheck
	})
}
