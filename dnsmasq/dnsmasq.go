package dnsmasq

import (
	"encoding/binary"
	"net"
	"strconv"

	"github.com/godbus/dbus"
	log "github.com/sirupsen/logrus"
)

const (
	dnsmasqService       string = "org.freedesktop.NetworkManager.dnsmasq"
	dnsmasqObjectPath    string = "/uk/org/thekelleys/dnsmasq"
	dnsmasqBaseInterface string = "uk.org.thekelleys."

	nmService       string = "org.freedesktop.NetworkManager"
	nmObjectPath    string = "/org/freedesktop/NetworkManager"
	nmBaseInterface string = nmService + "."

	nmActiveConnectionBaseInterface string = nmService + ".Connection.Active."
	nmIP4ConfigBaseInterface        string = nmService + ".IP4Config."

	propertiesBaseInterface string = "org.freedesktop.DBus.Properties."
)

var conn *dbus.Conn

// DNSServers a dns server with mapped domains
type DNSServers struct {
	ipaddress string
	zones     []string
}

// //NetworkManagerProperties a structure to receive NM properties
// type NetworkManagerProperties struct {
// 	Devices                 []dbus.ObjectPath
// 	AllDevices              []dbus.ObjectPath
// 	NetworkingEnabled       bool
// 	WirelessEnabled         bool
// 	WirelessHardwareEnabled bool
// 	WwanEnabled             bool
// 	WwanHardwareEnabled     bool
// 	WimaxEnabled            bool
// 	WimaxHardwareEnabled    bool
// 	ActiveConnections       []dbus.ObjectPath
// 	PrimaryConnection       dbus.ObjectPath
// 	PrimaryConnectionType   string
// 	Metered                 uint32
// 	ActivatingConnection    dbus.ObjectPath
// 	Startup                 bool
// 	Version                 string
// 	State                   uint32
// 	Connectivity            uint32
// 	GlobalDnsConfiguration  map[string]interface{}
// }

//GetSystemDbus return a cached connection to SystemBus
func GetSystemDbus() (*dbus.Conn, error) {

	if conn != nil {
		return conn, nil
	}

	dconn, err := dbus.SystemBus()
	if err != nil {
		log.Errorf("Failed connection to DBus: %s", err.Error())
		return nil, err
	}

	conn = dconn
	log.Debug("DBus connected")
	return conn, nil
}

//UpdateDNSServer map servers with domain matching to dnsmasq
func UpdateDNSServer(ips []string, port int, domains []string) (err error) {

	list := make([][]string, 0)

	for i := 0; i < len(ips); i++ {

		ns := ips[i]
		if port != 53 {
			ns += "#" + strconv.Itoa(port)
		}

		domainNS := []string{ns}
		for d := 0; d < len(domains); d++ {
			domainNS = append(domainNS, domains[d])
		}

		list = append(list, domainNS)
	}

	dnsServers, err := GetDNSServers()
	if err != nil {
		return err
	}
	for i := 0; i < len(dnsServers); i++ {
		list = append(list, []string{dnsServers[i]})
	}

	log.Debugf("Setting dns %v", list)

	//See also https://github.com/imp/dnsmasq/blob/master/dbus/DBus-interface#L149
	err = conn.Object(dnsmasqService, dbus.ObjectPath(dnsmasqObjectPath)).Call(dnsmasqBaseInterface+"SetServersEx", 0, list).Store()
	if err != nil {
		log.Errorf("Failed to update DNS servers: %s", err.Error())
	}

	return err
}

//ResetDNSServer reset dnsmasq dns to the connection defaults
func ResetDNSServer() (err error) {

	list := make([][]string, 0)

	dnsServers, err := GetDNSServers()
	if err != nil {
		return err
	}
	for i := 0; i < len(dnsServers); i++ {
		list = append(list, []string{dnsServers[i]})
	}

	log.Debugf("Setting dns %v", list)

	err = conn.Object(dnsmasqService, dbus.ObjectPath(dnsmasqObjectPath)).Call(dnsmasqBaseInterface+"SetServers", 0, list).Store()
	if err != nil {
		log.Errorf("Failed to update DNS servers: %s", err.Error())
	}

	return err
}

//GetDNSServers return a list of DNSservers in use by the system
func GetDNSServers() ([]string, error) {

	conn, err := GetSystemDbus()
	if err != nil {
		return nil, err
	}

	log.Debug("Loading DNS settings")

	ip4ConfigObjectPath, err := getIP4Config()
	if err != nil {
		return nil, err
	}

	ipList := make([]string, 0)

	val, err := conn.Object(nmService, *ip4ConfigObjectPath).GetProperty(nmIP4ConfigBaseInterface + "Nameservers")
	if err != nil {
		log.Errorf("Failed to query %s DBus %s", *ip4ConfigObjectPath, err.Error())
		return nil, err
	}

	rawns := val.Value().([]uint32)
	cnt := len(rawns)

	for i := 0; i < cnt; i++ {
		ipList = append(ipList, int2ip(rawns[i]).String())
	}

	log.Debugf("Found %d nameservers", cnt)

	return ipList, err
}

//GetIPs return a list of exposed IPs
func GetIPs() ([]string, error) {

	conn, err := GetSystemDbus()
	if err != nil {
		return nil, err
	}

	log.Debug("Loading IPs")

	ip4ConfigObjectPath, err := getIP4Config()
	if err != nil {
		return nil, err
	}

	ipList := make([]string, 0)

	val, err := conn.Object(nmService, *ip4ConfigObjectPath).GetProperty(nmIP4ConfigBaseInterface + "Addresses")
	if err != nil {
		log.Errorf("Failed to query %s DBus %s", *ip4ConfigObjectPath, err.Error())
		return nil, err
	}

	rawip := val.Value().([][]uint32)
	cnt := len(rawip)

	for ii := 0; ii < cnt; ii++ {
		for i := 0; i < len(rawip[ii]); i++ {
			ipList = append(ipList, int2ip(rawip[ii][i]).String())
		}
	}

	log.Debugf("Found %d ips", cnt)

	return ipList, err
}

func getIP4Config() (*dbus.ObjectPath, error) {

	conn, err := GetSystemDbus()
	if err != nil {
		return nil, err
	}

	val, err := conn.Object(nmService, dbus.ObjectPath(nmObjectPath)).GetProperty(nmBaseInterface + "PrimaryConnection")
	if err != nil {
		log.Errorf("Failed to query %s DBus %s", nmObjectPath, err.Error())
		return nil, err
	}

	if &val == nil {
		log.Errorf("Internet connection not available, cannot find default DNS")
		return nil, nil
	}

	activeConnObjectPath := val.Value().(dbus.ObjectPath)
	val, err = conn.Object(nmService, activeConnObjectPath).GetProperty(nmActiveConnectionBaseInterface + "Ip4Config")
	if err != nil {
		log.Errorf("Failed to query %s DBus %s", activeConnObjectPath, err.Error())
		return nil, err
	}

	ip4ConfigObjectPath := val.Value().(dbus.ObjectPath)

	return &ip4ConfigObjectPath, nil
}

func ip2int(ip net.IP) uint32 {
	if len(ip) == 16 {
		return binary.LittleEndian.Uint32(ip[12:16])
	}
	return binary.LittleEndian.Uint32(ip)
}

func int2ip(nn uint32) net.IP {
	ip := make(net.IP, 4)
	binary.LittleEndian.PutUint32(ip, nn)
	return ip
}
