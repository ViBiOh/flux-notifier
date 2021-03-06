package grafana

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"strings"

	"github.com/ViBiOh/httputils/v4/pkg/flags"
	"github.com/ViBiOh/httputils/v4/pkg/httperror"
	"github.com/ViBiOh/httputils/v4/pkg/httpjson"
	"github.com/ViBiOh/httputils/v4/pkg/logger"
	"github.com/ViBiOh/httputils/v4/pkg/request"
	"github.com/fluxcd/pkg/recorder"
)

type annotationPayload struct {
	Text string
	Tags []string
}

// App of package
type App interface {
	Handler() http.Handler
}

// Config of package
type Config struct {
	address  *string
	username *string
	password *string
}

type app struct {
	address  string
	username string
	password string
}

// Flags adds flags for configuring package
func Flags(fs *flag.FlagSet, prefix string) Config {
	return Config{
		address:  flags.New(prefix, "grafana").Name("Address").Default("http://grafana").Label("Address").ToString(fs),
		username: flags.New(prefix, "grafana").Name("Username").Default("").Label("Username for auth").ToString(fs),
		password: flags.New(prefix, "grafana").Name("Password").Default("").Label("Password for auth").ToString(fs),
	}
}

// New creates new App from Config
func New(config Config) App {
	return app{
		address:  fmt.Sprintf("%s/api/annotations", strings.TrimSpace(*config.address)),
		username: strings.TrimSpace(*config.username),
		password: strings.TrimSpace(*config.password),
	}
}

// Handler for Hello request. Should be use with net/http
func (a app) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var event recorder.Event
		if err := httpjson.Parse(r, &event); err != nil {
			httperror.InternalServerError(w, fmt.Errorf("unable to parse event: %s", err))
			return
		}

		w.WriteHeader(http.StatusOK)
		a.send(context.Background(), strings.TrimSpace(event.Message), event.InvolvedObject.Kind, event.InvolvedObject.Namespace, event.InvolvedObject.Name, event.Severity)
	})
}

func (a app) send(ctx context.Context, text string, tags ...string) {
	if strings.HasPrefix(text, "no update") || len(text) > 255 {
		return
	}

	req := request.New().Post(a.address)
	if len(a.username) != 0 {
		req.BasicAuth(a.username, a.password)
	}

	resp, err := req.JSON(ctx, annotationPayload{
		Text: text,
		Tags: tags,
	})

	if err != nil {
		logger.Error("%s", err)
		return
	}

	body, err := request.ReadBodyResponse(resp)
	if err != nil {
		logger.Error("%s", err)
		return
	}

	logger.Info("Grafana annotation succeeded: %s", body)
}
