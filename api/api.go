package api

import (
	"net"
	"net/http"

	"golang.org/x/net/context"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

type ddnsServer struct{}

func newDDNSServer() DDNSServiceServer {
	return new(ddnsServer)
}

func (s *ddnsServer) DeleteRecord(ctx context.Context, msg *Record) (*Record, error) {
	log.Debugf("Delete request %s", msg.GetDomain())
	return msg, nil
}

func (s *ddnsServer) SaveRecord(ctx context.Context, msg *Record) (*Record, error) {
	log.Debugf("Save request %s", msg.GetDomain())
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
func RunEndPoint(iface string, address string, opts ...runtime.ServeMuxOption) error {

	log.Debugf("Starting gw %s -> %s", iface, address)

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	mux := runtime.NewServeMux(opts...)
	dialOpts := []grpc.DialOption{grpc.WithInsecure()}
	err := RegisterDDNSServiceHandlerFromEndpoint(ctx, mux, iface, dialOpts)
	if err != nil {
		return err
	}

	http.ListenAndServe(address, mux)
	return nil
}
