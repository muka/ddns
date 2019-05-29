package main

import (
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/miekg/dns"
	"github.com/muka/ddns/api"
	coredns_grpc "github.com/muka/ddns/coredns"
	"github.com/muka/ddns/db"
	ddns "github.com/muka/ddns/dns"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"google.golang.org/grpc"
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
			Usage:  "Bind to specified IP",
			EnvVar: "IP",
		},
		cli.StringFlag{
			Name:   "coredns, c",
			Value:  "",
			Usage:  "Expose CoreDNS gRPC endpoint (will disable internal DNS)",
			EnvVar: "COREDNS",
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
		port := c.Int("port")
		httpServer := c.String("http-server")
		grpcEndpoint := c.String("grpc-server")
		coreDNSEndpoint := c.String("coredns")

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
		dns.HandleFunc(".", func(w dns.ResponseWriter, request *dns.Msg) {
			response := ddns.HandleDNSRequest(request)
			w.WriteMsg(response)
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

		if coreDNSEndpoint != "" {
			go func() {
				coreDNSGRPC(coreDNSEndpoint)
			}()
		} else {
			// Start internal DNS server
			go ddns.Serve(ip, port)
		}

		// scheduler()
		ticker := scheduler()
		defer ticker.Stop()

		waitSignal()

		return nil
	}

	app.Run(os.Args)
}

func coreDNSGRPC(endpoint string) error {
	lis, err := net.Listen("tcp", endpoint)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	api.RegisterDnsServiceServer(grpcServer, &coredns_grpc.DnsServer{})
	defer grpcServer.GracefulStop()
	return grpcServer.Serve(lis)
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
			quit = true
			break
		}
		if quit {
			break
		}
	}

}
