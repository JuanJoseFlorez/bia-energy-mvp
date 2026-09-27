package httpx

import (
	"fmt"
	"net/http"
)

// Routes wraps mux so unmatched requests are classified correctly.
// http.ServeMux itself already answers a wrong method on a known path with a plain
// 405 and an unmatched path with a plain 404, but both cases report an empty pattern
// from mux.Handler, so they cannot be told apart before running the handler. Routes
// runs the unmatched request through mux's own handler into a capturing response
// writer: a 405 becomes the standard JSON error body with the mux-computed Allow
// header copied over, anything else becomes NotFound.
func Routes(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, pattern := mux.Handler(r); pattern != "" {
			mux.ServeHTTP(w, r)
			return
		}

		h, _ := mux.Handler(r)
		probe := &routeProbe{}
		h.ServeHTTP(probe, r)

		if probe.status == http.StatusMethodNotAllowed {
			if allow := probe.Header().Get("Allow"); allow != "" {
				w.Header().Set("Allow", allow)
			}
			msg := fmt.Sprintf("method %s not allowed on %s", r.Method, r.URL.Path)
			writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", msg)
			return
		}
		NotFound(w, r)
	})
}

// routeProbe captures the status and headers a handler writes without sending
// anything to the real client; it is used to peek at ServeMux's own unmatched-route
// handler and discern a 405 from a 404 before deciding how to answer the real w.
type routeProbe struct {
	header http.Header
	status int
}

func (p *routeProbe) Header() http.Header {
	if p.header == nil {
		p.header = make(http.Header)
	}
	return p.header
}

func (p *routeProbe) Write(b []byte) (int, error) {
	if p.status == 0 {
		p.status = http.StatusOK
	}
	return len(b), nil
}

func (p *routeProbe) WriteHeader(status int) {
	if p.status == 0 {
		p.status = status
	}
}
