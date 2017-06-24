package main

import (
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/miekg/dns"
	"github.com/muka/dyndns/api"
	"github.com/muka/dyndns/db"
	ddns "github.com/muka/dyndns/dns"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func main() {

	app := cli.NewApp()

	app.Flags = []cli.Flag{
		// cli.StringFlag{
		// 	Name:   "logfile, l",
		// 	Value:  "./data/logs",
		// 	Usage:  "path to logfile",
		// 	EnvVar: "LOGFILE",
		// },
		cli.StringFlag{
			Name:   "tsig, t",
			Value:  "",
			Usage:  "use MD5 hmac tsig: keyname:base64",
			EnvVar: "TSIG",
		},
		cli.StringFlag{
			Name:   "http-server, s",
			Value:  ":5551",
			Usage:  "host:port combination to bind the http service to",
			EnvVar: "HTTP",
		},
		cli.StringFlag{
			Name:   "grpc-server, g",
			Value:  ":50551",
			Usage:  "host:port combination to bind the grpc service to",
			EnvVar: "GRPC",
		},
		cli.StringFlag{
			Name:   "dbpath, d",
			Value:  "./data/ddns.db",
			Usage:  "location where db will be stored",
			EnvVar: "DBPATH",
		},
		cli.IntFlag{
			Name:   "port, p",
			Value:  10053,
			Usage:  "DNS server port",
			EnvVar: "PORT",
		},
		cli.BoolFlag{
			Name:   "debug",
			Usage:  "Enable debug",
			EnvVar: "DEBUG",
		},
	}

	app.Action = func(c *cli.Context) error {

		// logfile := c.String("logfile")
		debug := c.Bool("debug")
		tsig := c.String("tsig")
		dbPath := c.String("dbpath")
		port := c.Int("port")
		httpServer := c.String("http-server")
		grpcEndpoint := c.String("grpc-server")

		if debug {
			log.SetLevel(log.DebugLevel)
		}

		var (
			name   string // tsig keyname
			secret string // tsig base64
		)

		log.Debugf("Connecting to %s", dbPath)
		err1 := db.Connect(dbPath)
		if err1 != nil {
			panic(err1)
		}
		defer db.Disconnect()

		enableUpdates := false
		// Tsig extract
		log.Debug("Check for TSIG")
		if tsig != "" {
			a := strings.SplitN(tsig, ":", 2)
			name, secret = dns.Fqdn(a[0]), a[1]
			enableUpdates = true
		}

		// Attach request handler func
		log.Debug("Attaching DNS handler")
		dns.HandleFunc(".", func(w dns.ResponseWriter, r *dns.Msg) {
			ddns.HandleDNSRequest(w, r, enableUpdates)
		})

		log.Debug("Starting services")
		go func() {
			if err := api.Run(grpcEndpoint); err != nil {
				panic(err.Error())
			}
		}()
		go func() {
			if err := api.RunEndPoint(grpcEndpoint, httpServer); err != nil {
				panic(err.Error())
			}
		}()

		// Start server
		go ddns.Serve(name, secret, port)

		waitSignal()

		return nil
	}

	app.Run(os.Args)
}

func waitSignal() {

	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	quit := false
	for {
		select {
		case s := <-sig:
			log.Printf("Signal (%d) received, stopping", s)
			quit = true
			break
		}
		if quit {
			break
		}
	}

}
