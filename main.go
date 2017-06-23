package main

import (
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/miekg/dns"
	"github.com/muka/dyndns/api"
	"github.com/muka/dyndns/db"
	ddns "github.com/muka/dyndns/dns"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func parseQuery(m *dns.Msg) {
	var rr dns.RR
	for _, q := range m.Question {
		if readRR, e := ddns.GetRecord(q.Name, q.Qtype); e == nil {
			rr = readRR.(dns.RR)
			if rr.Header().Name == q.Name {
				m.Answer = append(m.Answer, rr)
			}
		}
	}
}

func handleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {

	m := new(dns.Msg)
	m.SetReply(r)
	m.Compress = false

	switch r.Opcode {
	case dns.OpcodeQuery:
		log.Debugf("Got query request")
		parseQuery(m)

	case dns.OpcodeUpdate:
		log.Debugf("Got update request")
		for _, question := range r.Question {
			for _, rr := range r.Ns {
				ddns.UpdateRecord(rr, &question)
			}
		}
	}

	if r.IsTsig() != nil {
		if w.TsigStatus() == nil {
			m.SetTsig(r.Extra[len(r.Extra)-1].(*dns.TSIG).Hdr.Name,
				dns.HmacMD5, 300, time.Now().Unix())
		} else {
			log.Println("Status ", w.TsigStatus().Error())
		}
	}

	w.WriteMsg(m)
}

func serve(name, secret string, port int) error {

	log.Debugf("Starting server on :%d", port)
	server := &dns.Server{Addr: ":" + strconv.Itoa(port), Net: "udp"}

	if name != "" {
		server.TsigSecret = map[string]string{name: secret}
	}

	err := server.ListenAndServe()
	defer server.Shutdown()

	if err != nil {
		log.Fatalf("Failed to setup the udp server: %s", err.Error())
	}

	return err
}

func main() {

	app := cli.NewApp()

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "logfile, l",
			Value:  "./data/logs",
			Usage:  "path to logfile",
			EnvVar: "LOGFILE",
		},
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
	}

	app.Action = func(c *cli.Context) error {

		log.SetLevel(log.DebugLevel)

		// logfile := c.String("logfile")
		tsig := c.String("tsig")
		dbPath := c.String("dbpath")
		port := c.Int("port")

		httpServer := c.String("http-server")
		grpcEndpoint := c.String("grpc-server")

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

		// Attach request handler func
		log.Debug("Attaching DNS handler")
		dns.HandleFunc(".", handleDNSRequest)

		// Tsig extract
		log.Debug("Check for TSIG")
		if tsig != "" {
			a := strings.SplitN(tsig, ":", 2)
			name, secret = dns.Fqdn(a[0]), a[1]
		}

		// Start server
		go serve(name, secret, port)

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

		return nil
	}

	app.Run(os.Args)
}
