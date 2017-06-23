package dns

import (
	"errors"
	"math"
	"net"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/miekg/dns"
	"github.com/muka/dyndns/db"
)

//GetKey return the reverse domain
func GetKey(domain string, rtype uint16) (r string, e error) {
	log.Debugf("Get key for %s", domain)
	if n, ok := dns.IsDomainName(domain); ok {

		labels := dns.SplitDomainName(domain)

		// Reverse domain, starting from top-level domain
		// eg.  ".com.mkaczanowski.test "
		var tmp string
		for i := 0; i < int(math.Floor(float64(n/2))); i++ {
			tmp = labels[i]
			labels[i] = labels[n-1]
			labels[n-1] = tmp
		}

		reverseDomain := strings.Join(labels, ".")
		r = strings.Join([]string{reverseDomain, strconv.Itoa(int(rtype))}, "_")
		log.Debugf("Key is %s", r)
	} else {
		e = errors.New("Invalid domain:  " + domain)
		log.Error(e.Error())
	}

	return r, e
}

//GetRecord return a new DNS record
func GetRecord(domain string, rtype uint16) (dns.RR, error) {

	log.Debugf("Load record %s", domain)

	key, err := GetKey(domain, rtype)
	if err != nil {
		return nil, err
	}

	val, err := db.GetRecord(key, rtype)
	if err == nil {
		return dns.NewRR(string(val))
	}

	return nil, err
}

//UpdateRecord update or remove a record
func UpdateRecord(r dns.RR, q *dns.Question) error {

	var (
		rr    dns.RR
		name  string
		rtype uint16
		ttl   uint32
		ip    net.IP
	)

	header := r.Header()
	name = header.Name
	rtype = header.Rrtype
	ttl = header.Ttl

	revName, err := GetKey(name, rtype)

	if err != nil {
		return err
	}

	if _, ok := dns.IsDomainName(name); ok {
		if header.Class == dns.ClassANY && header.Rdlength == 0 { // Delete record
			log.Debugf("Remove %s", name)
			db.DeleteRecord(revName, rtype)
		} else { // Add record
			rheader := dns.RR_Header{
				Name:   name,
				Rrtype: rtype,
				Class:  dns.ClassINET,
				Ttl:    ttl,
			}

			if a, ok := r.(*dns.A); ok {
				rrr, err := GetRecord(name, rtype)
				if err == nil {
					rr = rrr.(*dns.A)
				} else {
					rr = new(dns.A)
				}

				ip = a.A
				rr.(*dns.A).Hdr = rheader
				rr.(*dns.A).A = ip
			} else if a, ok := r.(*dns.AAAA); ok {
				rrr, err := GetRecord(name, rtype)
				if err == nil {
					rr = rrr.(*dns.AAAA)
				} else {
					rr = new(dns.AAAA)
				}

				ip = a.AAAA
				rr.(*dns.AAAA).Hdr = rheader
				rr.(*dns.AAAA).AAAA = ip
			}

			rrKey, err1 := GetKey(rr.Header().Name, rr.Header().Rrtype)
			if err1 != nil {
				return err1
			}
			rrBody := rr.String()

			log.Debugf("Saving record %s (%s)", rr.Header().Name, rrKey)
			db.StoreRecord(rrKey, rrBody)
		}
	}

	return nil
}
