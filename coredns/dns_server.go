package coredns

import (
	"context"
	"fmt"

	"github.com/miekg/dns"
	"github.com/muka/ddns/api"
	ddns_dns "github.com/muka/ddns/dns"
)

type DnsServer struct{}

func (d *DnsServer) Query(ctx context.Context, in *api.DnsPacket) (*api.DnsPacket, error) {

	request := new(dns.Msg)
	if err := request.Unpack(in.Msg); err != nil {
		return nil, fmt.Errorf("failed to unpack msg: %v", err)
	}

	response := ddns_dns.HandleDNSRequest(request)

	out, err := response.Pack()
	if err != nil {
		return nil, fmt.Errorf("failed to pack msg: %v", err)
	}

	return &api.DnsPacket{Msg: out}, nil
}
