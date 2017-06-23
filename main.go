package main

import (
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/miekg/dns"
	"github.com/muka/dyndns/db"
	ddns "github.com/muka/dyndns/dns"
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
		parseQuery(m)

	case dns.OpcodeUpdate:
		for _, question := range r.Question {
			for _, rr := range r.Ns {
				updateRecord(rr, &question)
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

func serve(name, secret string, port int) {

	server := &dns.Server{Addr: ": " + strconv.Itoa(port), Net: "udp"}

	if name != "" {
		server.TsigSecret = map[string]string{name: secret}
	}

	err := server.ListenAndServe()
	defer server.Shutdown()

	if err != nil {
		log.Fatalf("Failed to setup the udp server: %sn ", err.Error())
	}
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
			Name:   "dbpath, d",
			Value:  "./data/ddns.db",
			Usage:  "location where db will be stored",
			EnvVar: "DBPATH",
		},
		cli.IntFlag{
			Name:   "port, p",
			Value:  53,
			Usage:  "DNS server port",
			EnvVar: "PORT",
		},
		cli.StringFlag{
			Name:   "pid",
			Value:  "./data/ddns.pid",
			Usage:  "pid file location",
			EnvVar: "PID",
		},
	}

	app.Action = func(c *cli.Context) error {

		logfile := c.String("logfile")
		tsig := c.String("tsig")
		dbPath := c.String("dbpath")
		port := c.Int("port")
		pidFile := c.String("pid")

		var (
			name   string   // tsig keyname
			secret string   // tsig base64
			fh     *os.File // logfile handle
		)

		err := db.Connect(dbPath)
		if err != nil {
			panic(err)
		}
		defer db.Disconnect()

		// Attach request handler func
		dns.HandleFunc(".", handleDNSRequest)

		// Tsig extract
		if tsig != "" {
			a := strings.SplitN(tsig, ":", 2)
			name, secret = dns.Fqdn(a[0]), a[1]
		}

		// Logger setup
		if logfile != "" {
			if _, err := os.Stat(logfile); os.IsNotExist(err) {
				if file, err := os.Create(logfile); err != nil {
					if err != nil {
						log.Panic("Couldn't create log file: ", err)
					}

					fh = file
				}
			} else {
				fh, _ = os.OpenFile(logfile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
			}
			defer fh.Close()
			log.SetOutput(fh)
		}

		// Pidfile
		file, err := os.OpenFile(pidFile, os.O_RDWR|os.O_CREATE, 0666)
		if err != nil {
			log.Panic("Couldn't create pid file: ", err)
		} else {
			file.Write([]byte(strconv.Itoa(syscall.Getpid())))
			defer file.Close()
		}

		// Start server
		go serve(name, secret, port)

		sig := make(chan os.Signal)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		for {
			select {
			case s := <-sig:
				log.Printf("Signal (%d) received, stopping", s)
				break
			}
		}

		return nil
	}

	app.Run(os.Args)
}
