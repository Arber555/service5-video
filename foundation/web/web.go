package web

import (
	"context"
	"errors"
	"net/http"
	"os"
	"syscall"
	"time"

	"github.com/dimfeld/httptreemux/v5"
	"github.com/google/uuid"
)

// A Handler is a type that handles a http request within our own little mini
// framework.
type Handler func(ctx context.Context, w http.ResponseWriter, r *http.Request) error

// App is the entrypoint into our application and what configures our context
// object for each of our http handlers. Feel free to add any configuration
// data/logic on this App struct.
type App struct {
	*httptreemux.ContextMux
	shutdown chan os.Signal
	mv       []Middleware
}

// NewApp creates an App value that handle a set of routes for the application.
func NewApp(shutdown chan os.Signal, mv ...Middleware) *App {
	return &App{
		ContextMux: httptreemux.NewContextMux(),
		shutdown:   shutdown,
		mv:         mv,
	}
}

// SignalShutdown is used to gracefully shut down the app when an integrity
// issue is identified.
func (a *App) SignalShutdown() {
	a.shutdown <- syscall.SIGTERM
}

// Handle sets a handler function for a given HTTP method and path pair
// to the application server mux.
func (a *App) Handle(method string, path string, handler Handler, mv ...Middleware) {
	handler = wrapMiddleware(mv, handler)
	handler = wrapMiddleware(a.mv, handler)

	h := func(w http.ResponseWriter, r *http.Request) {

		v := Values{
			TraceID: uuid.NewString(),
			Now:     time.Now().UTC(),
		}
		ctx := SetValues(r.Context(), &v)

		if err := handler(ctx, w, r); err != nil {
			if validateShutdown(err) {
				a.SignalShutdown()
				return
			}
		}
	}

	a.ContextMux.Handle(method, path, h)
}

// validateShutdown validates the error for special conditions that do not
// warrant an actual shutdown by the system.
func validateShutdown(err error) bool {

	switch {
	case errors.Is(err, syscall.EPIPE):
		return false
	case errors.Is(err, syscall.ECONNRESET):
		return false
	}

	return true
}
