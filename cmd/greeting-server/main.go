package main

import (
	"fmt"
	"net/http"
	"os"

	log "github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
)

func main() {
	app := cli.NewApp()
	app.Name = "Greeting"
	app.Usage = "Just another greeting server"
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "bind",
			Usage:   "Binding address for HTTP server",
			Value:   ":80",
			Aliases: []string{"b"},
			EnvVars: []string{"BIND"},
		},
		&cli.StringFlag{
			Name:    "name",
			Usage:   "Greeting name for the server",
			Value:   "anonymous",
			Aliases: []string{"n"},
			EnvVars: []string{"NAME"},
		},
	}
	app.Action = func(ctx *cli.Context) error {
		addr := ctx.String("bind")
		name := ctx.String("name")
		server := GreetingServer{Name: name}
		http.HandleFunc("/health", server.HandleHealthcheck)
		http.HandleFunc("/greet", server.HandleGreet)
		log.WithField("addr", addr).WithField("name", name).Info("Starting listening")
		return http.ListenAndServe(addr, nil)
	}

	if err := app.Run(os.Args); err != nil {
		log.WithError(err).Fatal("Unable to start application")
	}
}

// GreetingServer is capable of presenting itself thanks to HTTP handlers.
type GreetingServer struct {
	// Name is the server name.
	Name string
}

// HandleGreet is a HTTP handler answering the server name.
func (s GreetingServer) HandleGreet(rw http.ResponseWriter, req *http.Request) {
	log.Debug("Greet")
	body := fmt.Sprintf("I am %s", s.Name)
	if _, err := rw.Write([]byte(body)); err != nil {
		log.WithError(err).Warning("Unable to write greeting content")
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
}

// HandleHealthcheck returns 200 Ok.
func (s GreetingServer) HandleHealthcheck(rw http.ResponseWriter, req *http.Request) {
	log.Debug("Health check")
	rw.WriteHeader(http.StatusOK)
}
