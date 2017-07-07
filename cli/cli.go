package main

import (
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/miekg/dns"
	"github.com/muka/ddns/api"
	"github.com/muka/ddns/db"
	ddns "github.com/muka/ddns/dns"
	"github.com/muka/ddns/dnsmasq"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

const timerSeconds = 15

func main() {

	app := cli.NewApp()

	app.Flags = []cli.Flag{
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
		cli.StringFlag{
			Name:   "ip, i",
			Value:  "",
			Usage:  "DNS ip",
			EnvVar: "PORT",
		},
		cli.StringFlag{
			Name:   "zones, z",
			Usage:  "Comma-separated list of managed domain zones",
			EnvVar: "ZONES",
		},
		cli.BoolFlag{
			Name:   "debug",
			Usage:  "Enable debug",
			EnvVar: "DEBUG",
		},
	}

	app.Action = func(c *cli.Context) error {

		debug := c.Bool("debug")
		dbPath := c.String("dbpath")
		ip := c.String("ip")
		zones := c.String("zones")
		port := c.Int("port")
		httpServer := c.String("http-server")
		grpcEndpoint := c.String("grpc-server")

		if debug {
			log.SetLevel(log.DebugLevel)
		}

		log.Debugf("Connecting to %s", dbPath)
		err1 := db.Connect(dbPath)
		if err1 != nil {
			panic(err1)
		}
		defer db.Disconnect()

		// Attach request handler func
		log.Debug("Attaching DNS handler")
		dns.HandleFunc(".", func(w dns.ResponseWriter, r *dns.Msg) {
			ddns.HandleDNSRequest(w, r)
		})

		var ips []string
		var err error
		if ip == "" {
			ips, err = dnsmasq.GetIPs()
			if err != nil {
				log.Errorf("Failed to load bindable IPs %s", err.Error())
				return err
			}
		} else {
			ips = []string{ip}
		}

		if zones != "" {
			log.Debug("Updating host DNS")
			domains := strings.Split(zones, ",")
			dnsmasq.UpdateDNSServer(ips, port, domains)
		} else {
			log.Warnf("No zones specified, dnsmasq integration will be disabled")
		}

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
		go ddns.Serve(ip, port)

		scheduler()
		// ticker := scheduler()
		// ticker.Stop()

		waitSignal()

		return nil
	}

	app.Run(os.Args)
}

func scheduler() *time.Ticker {

	ticker := time.NewTicker(time.Second * timerSeconds)
	go func() {
		for range ticker.C {
			ddns.RemoveExpired()
		}
	}()

	return ticker
}

func waitSignal() {

	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	quit := false
	for {
		select {
		case s := <-sig:
			log.Printf("Signal (%d) received, stopping", s)
			dnsmasq.ResetDNSServer()
			quit = true
			break
		}
		if quit {
			break
		}
	}

}
