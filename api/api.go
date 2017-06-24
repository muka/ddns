package api

import (
	"errors"
	"net"
	"net/http"
	"strings"

	"golang.org/x/net/context"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/miekg/dns"
	"github.com/muka/dyndns/db"
	ddns "github.com/muka/dyndns/dns"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

type ddnsServer struct{}

func newDDNSServer() DDNSServiceServer {
	return new(ddnsServer)
}

func (s *ddnsServer) DeleteRecord(ctx context.Context, msg *Record) (*Record, error) {
	log.Debugf("Delete request %s %s", msg.GetType(), msg.GetDomain())
	return msg, nil
}

func (s *ddnsServer) SaveRecord(ctx context.Context, msg *Record) (*Record, error) {

	log.Debugf("Save request: %s %s %s", msg.GetType(), msg.GetDomain(), msg.GetIp())

	var ttl uint32 = 120
	if msg.GetTTL() > 0 {
		ttl = uint32(msg.GetTTL())
	}

	if msg.GetIp() == "" {
		return nil, errors.New("IP is missing")
	}
	if msg.GetDomain() == "" {
		return nil, errors.New("Domain is missing")
	}

	var rtype uint16
	var rr dns.RR

	switch strings.ToUpper(msg.GetType()) {
	case "A":

		rtype = dns.TypeA
		ipaddress := net.IP(msg.GetIp())

		rr = new(dns.A)
		rr.(*dns.A).A = ipaddress
		rr.(*dns.A).Hdr = ddns.GetHeader(msg.GetDomain(), rtype, ttl)

		break
	case "AAAA":

		rtype = dns.TypeAAAA
		ipaddress := net.IP(msg.GetIp())

		rr = new(dns.AAAA)
		rr.(*dns.AAAA).AAAA = ipaddress
		rr.(*dns.AAAA).Hdr = ddns.GetHeader(msg.GetDomain(), rtype, ttl)

		break
	case "MX":

		rtype = dns.TypeMX

		rr = new(dns.MX)
		rr.(*dns.MX).Mx = msg.GetIp()
		rr.(*dns.MX).Hdr = ddns.GetHeader(msg.GetDomain(), rtype, ttl)
		rr.(*dns.MX).Preference = 0

		break

	case "CNAME":

		rtype = dns.TypeCNAME

		rr = new(dns.CNAME)
		rr.(*dns.CNAME).Target = msg.GetIp()
		rr.(*dns.CNAME).Hdr = ddns.GetHeader(msg.GetDomain(), rtype, ttl)

		break
	}

	if rtype == 0 {
		return nil, errors.New("Record type not supported (Use one of A, AAAA, MX, CNAME)")
	}

	key, err := ddns.GetKey(msg.GetDomain(), rtype)
	if err != nil {
		return nil, err
	}

	record := db.Record{
		Expires: int64(msg.GetExpires()),
		RR:      rr.String(),
	}

	err = db.StoreRecord(key, record)

	if err != nil {
		return nil, err
	}

	if msg.GetPTR() {
		// Add PTR record
		err := ddns.AddPTRRecord(msg.GetIp(), msg.GetDomain(), ttl, int64(msg.GetExpires()))
		if err != nil {
			return nil, err
		}
	}

	return msg, nil
}

//Run start the server
func Run(iface string) error {
	log.Debugf("Listening gRPC service at %s", iface)
	listen, err := net.Listen("tcp", iface)
	if err != nil {
		return err
	}
	server := grpc.NewServer()
	RegisterDDNSServiceServer(server, newDDNSServer())
	server.Serve(listen)
	return nil
}

// RunEndPoint start the JSON restful api
func RunEndPoint(grpcAddress string, address string, opts ...runtime.ServeMuxOption) error {

	log.Debugf("Starting JSON API %s", address)

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	mux := runtime.NewServeMux(opts...)
	dialOpts := []grpc.DialOption{grpc.WithInsecure()}
	err := RegisterDDNSServiceHandlerFromEndpoint(ctx, mux, grpcAddress, dialOpts)
	if err != nil {
		return err
	}

	http.ListenAndServe(address, mux)
	return nil
}
