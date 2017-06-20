package main

import (
	"errors"
	"flag"
	"log"
	"math"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/boltdb/bolt"
	"github.com/miekg/dns"
)

var (
	tsig     *string
	db_path  *string
	port     *int
	bdb      *bolt.DB
	logfile  *string
	pid_file *string
)

const rr_bucket = "rr"

func getKey(domain string, rtype uint16) (r string, e error) {
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

		reverse_domain := strings.Join(labels, ".")
		r = strings.Join([]string{reverse_domain, strconv.Itoa(int(rtype))}, "_ ")
	} else {
		e = errors.New("Invailid domain:  " + domain)
		log.Println(e.Error())
	}

	return r, e
}

func createBucket(bucket string) (err error) {
	err = bdb.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucket))
		if err != nil {
			e := errors.New("Create bucket:  " + bucket)
			log.Println(e.Error())

			return e
		}

		return nil
	})

	return err
}

func deleteRecord(domain string, rtype uint16) (err error) {
	key, _ := getKey(domain, rtype)
	err = bdb.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(rr_bucket))
		err := b.Delete([]byte(key))

		if err != nil {
			e := errors.New("Delete record failed for domain:  " + domain)
			log.Println(e.Error())

			return e
		}

		return nil
	})

	return err
}

func storeRecord(rr dns.RR) (err error) {
	key, _ := getKey(rr.Header().Name, rr.Header().Rrtype)
	err = bdb.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(rr_bucket))
		err := b.Put([]byte(key), []byte(rr.String()))

		if err != nil {
			e := errors.New("Store record failed:  " + rr.String())
			log.Println(e.Error())

			return e
		}

		return nil
	})

	return err
}

func getRecord(domain string, rtype uint16) (rr dns.RR, err error) {
	key, _ := getKey(domain, rtype)
	var v []byte

	err = bdb.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(rr_bucket))
		v = b.Get([]byte(key))

		if string(v) == "" {
			e := errors.New("Record not found, key:  " + key)
			log.Println(e.Error())

			return e
		}

		return nil
	})

	if err == nil {
		rr, err = dns.NewRR(string(v))
	}

	return rr, err
}

func updateRecord(r dns.RR, q *dns.Question) {
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

	if _, ok := dns.IsDomainName(name); ok {
		if header.Class == dns.ClassANY && header.Rdlength == 0 { // Delete record
			deleteRecord(name, rtype)
		} else { // Add record
			rheader := dns.RR_Header{
				Name:   name,
				Rrtype: rtype,
				Class:  dns.ClassINET,
				Ttl:    ttl,
			}

			if a, ok := r.(*dns.A); ok {
				rrr, err := getRecord(name, rtype)
				if err == nil {
					rr = rrr.(*dns.A)
				} else {
					rr = new(dns.A)
				}

				ip = a.A
				rr.(*dns.A).Hdr = rheader
				rr.(*dns.A).A = ip
			} else if a, ok := r.(*dns.AAAA); ok {
				rrr, err := getRecord(name, rtype)
				if err == nil {
					rr = rrr.(*dns.AAAA)
				} else {
					rr = new(dns.AAAA)
				}

				ip = a.AAAA
				rr.(*dns.AAAA).Hdr = rheader
				rr.(*dns.AAAA).AAAA = ip
			}

			storeRecord(rr)
		}
	}
}

func parseQuery(m *dns.Msg) {
	var rr dns.RR

	for _, q := range m.Question {
		if read_rr, e := getRecord(q.Name, q.Qtype); e == nil {
			rr = read_rr.(dns.RR)
			if rr.Header().Name == q.Name {
				m.Answer = append(m.Answer, rr)
			}
		}
	}
}

func handleDnsRequest(w dns.ResponseWriter, r *dns.Msg) {
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
	var (
		name   string   // tsig keyname
		secret string   // tsig base64
		fh     *os.File // logfile handle
	)

	// Parse flags
	logfile = flag.String("logfile", "", "path to log file")
	port = flag.Int("port", 53, "server port ")
	tsig = flag.String("tsig", "", "use MD5 hmac tsig: keyname:base64 ")
	db_path = flag.String("db_path", "./dyndns.db ", "location where db will be stored ")
	pid_file = flag.String("pid", "./go-dyndns.pid ", "pid file location ")

	flag.Parse()

	// Open db
	db, err := bolt.Open(*db_path, 0600,
		&bolt.Options{Timeout: 10 * time.Second})

	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	bdb = db

	// Create dns bucket if doesn't exist
	createBucket(rr_bucket)

	// Attach request handler func
	dns.HandleFunc(". ", handleDnsRequest)

	// Tsig extract
	if *tsig != "" {
		a := strings.SplitN(*tsig, ":", 2)
		name, secret = dns.Fqdn(a[0]), a[1]
	}

	// Logger setup
	if *logfile != "" {
		if _, err := os.Stat(*logfile); os.IsNotExist(err) {
			if file, err := os.Create(*logfile); err != nil {
				if err != nil {
					log.Panic("Couldn't create log file: ", err)
				}

				fh = file
			}
		} else {
			fh, _ = os.OpenFile(*logfile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		}
		defer fh.Close()
		log.SetOutput(fh)
	}

	// Pidfile
	file, err := os.OpenFile(*pid_file, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		log.Panic("Couldn't create pid file: ", err)
	} else {
		file.Write([]byte(strconv.Itoa(syscall.Getpid())))
		defer file.Close()
	}

	// Start server
	go serve(name, secret, *port)

	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
endless:
	for {
		select {
		case s := <-sig:
			log.Printf("Signal (%d) received, stopping", s)
			break endless
		}
	}
}
