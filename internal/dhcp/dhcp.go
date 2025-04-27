package dhcp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"slices"
	"strconv"
)

type DHCPConfig struct {
	IsEnable      bool   `json:"IsEnable"`
	Address       string `json:"Address"`
	FileName      string `json:"FileName"`
	RangeStart    string `json:"RangeStart"`
	DefaultRouter string `json:"DefaultRouter"`
	DNS           string `json:"DNS"`
}

type dhcp struct {
	op      byte
	htype   byte
	hlen    byte
	hops    byte
	xid     [4]byte
	secs    [2]byte
	flags   [2]byte
	ciaddr  [4]byte
	yiaddr  [4]byte
	siaddr  [4]byte
	giaddr  [4]byte
	chaddr  [16]byte
	sname   [64]byte
	file    [128]byte
	options []option
}

type option struct {
	code  byte
	len   byte
	value []byte
}

type leaseDB map[string]string

const udpMax = 65536
const leastMessageLen = 300

const BOOTREQUEST = 1
const BOOTREPLY = 2
const ETHERNET = 1
const ETHERNETHLEN = 6

const Pad = 0
const SubnetMask = 1
const Router = 3
const DomainServer = 6
const MTUInterface = 26
const BroadcastAddress = 28
const AddressTime = 51
const DHCPMsgType = 53
const DHCPServerId = 54
const ParameterList = 55
const ClassId = 60
const End = 255

const DHCPDISCOVER = 1
const DHCPOFFER = 2
const DHCPREQUEST = 3
const DHCPACK = 5

var logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

var fname string
var rangeStart string
var defaultRouter string
var dns string

var db leaseDB
var current byte
var serverId [4]byte

func Listen(conf DHCPConfig) error {
	fname = conf.FileName
	rangeStart = conf.RangeStart
	defaultRouter = conf.DefaultRouter
	dns = conf.DNS

	db = make(leaseDB)
	current = 0

	dummy, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return err
	}
	defer dummy.Close()
	copy(serverId[:], dummy.LocalAddr().(*net.UDPAddr).IP)

	conn, err := net.ListenPacket("udp", conf.Address)
	if err != nil {
		return err
	}

	go listen(conn)

	return nil
}

func listen(conn net.PacketConn) {
	defer conn.Close()

	rx := make([]byte, udpMax)
	for {
		n, _, err := conn.ReadFrom(rx)
		if err != nil {
			logger.Error(err.Error(), "module", "DHCP")
			continue
		}
		go handleDHCP(conn, slices.Clone(rx[:n]))
	}
}

func handleDHCP(conn net.PacketConn, p []byte) {
	dhcp, err := newdhcp(p)
	if err != nil {
		logger.Error(err.Error(), "module", "DHCP")
		return
	}

	tx := make([]byte, udpMax)
	switch dhcp.msgType() {
	case DHCPDISCOVER:
		logger.Info("receve DHCPDISCOVER", "module", "DHCP", "message", fmt.Sprintf("%v", dhcp))
		offer, err := dhcp.offer()
		if err != nil {
			logger.Error(err.Error(), "module", "DHCP")
			return
		}
		logger.Info("send DHCPOFFER", "module", "DHCP", "message", fmt.Sprintf("%v", offer))
		n, err := offer.write(tx)
		if err != nil {
			logger.Error(err.Error(), "module", "DHCP")
			return
		}
		tx = tx[:n]
	case DHCPREQUEST:
		logger.Info("receve DHCPREQUEST", "module", "DHCP", "message", fmt.Sprintf("%v", dhcp))
		ack, err := dhcp.ack()
		if err != nil {
			logger.Error(err.Error(), "module", "DHCP")
			return
		}
		logger.Info("send DHCPACK", "module", "DHCP", "message", fmt.Sprintf("%v", ack))
		n, err := ack.write(tx)
		if err != nil {
			logger.Error(err.Error(), "module", "DHCP")
			return
		}
		tx = tx[:n]
	default:
		logger.Info("receved message is not supported", "module", "DHCP", "message", fmt.Sprintf("%v", dhcp))
		return
	}

	addr, err := net.ResolveUDPAddr("udp", "255.255.255.255:68")
	if err != nil {
		logger.Error(err.Error(), "module", "DHCP")
		return
	}
	conn.WriteTo(slices.Clone(tx), addr)
}

func (d *dhcp) offer() (*dhcp, error) {
	return d.reply()
}

func (d *dhcp) ack() (*dhcp, error) {
	return d.reply()
}

func (d *dhcp) reply() (*dhcp, error) {
	msgType := option{
		code:  DHCPMsgType,
		len:   1,
		value: []byte{DHCPOFFER},
	}
	if d.msgType() == DHCPREQUEST {
		msgType = option{
			code:  DHCPMsgType,
			len:   1,
			value: []byte{DHCPACK},
		}
	}

	t := make([]byte, 4)
	binary.BigEndian.PutUint32(t, 864000)
	addressTime := option{
		code:  AddressTime,
		len:   4,
		value: t,
	}

	dhcpServerId := option{
		code:  DHCPServerId,
		len:   4,
		value: serverId[:],
	}

	o, err := options(d.parameterList())
	if err != nil {
		return nil, err
	}

	options := make([]option, 0, 3+len(o))
	options = append(options, msgType, addressTime, dhcpServerId)
	options = append(options, o...)

	yiaddr := [4]byte{0}
	pick, err := db.pick(d.chaddr)
	if err != nil {
		return nil, err
	}
	copy(yiaddr[:], pick[:])

	siaddr := [4]byte{0}
	file := [128]byte{0}
	if d.isPXE() {
		copy(siaddr[:], serverId[:])
		copy(file[:], []byte(fname))
	}

	return &dhcp{
		op:      BOOTREPLY,
		htype:   ETHERNET,
		hlen:    ETHERNETHLEN,
		hops:    0,
		xid:     d.xid,
		secs:    [2]byte{0},
		flags:   d.flags,
		ciaddr:  [4]byte{0},
		yiaddr:  yiaddr,
		siaddr:  siaddr,
		giaddr:  d.giaddr,
		chaddr:  d.chaddr,
		sname:   [64]byte{0},
		file:    file,
		options: options,
	}, nil
}

func newdhcp(p []byte) (*dhcp, error) {
	if [4]byte(p[236:240]) != [4]byte{99, 130, 83, 99} {
		return nil, errors.New("DHCP options have not magic number")
	}

	options := make([]option, 0)
	o := p[240:]
	i := 0
	for {
		if o[i] == End {
			break
		}
		if i >= len(o) {
			return nil, errors.New("DHCP options have not end option")
		}
		if o[i] == Pad {
			i++
			continue
		}
		code := o[i]
		len := o[i+1]
		value := o[i+2 : i+2+int(len)]
		options = append(options, option{code, len, value})
		i += 2 + int(len)
	}

	return &dhcp{
		op:      p[0],
		htype:   p[1],
		hlen:    p[2],
		hops:    p[3],
		xid:     [4]byte(p[4:8]),
		secs:    [2]byte(p[8:10]),
		flags:   [2]byte(p[10:12]),
		ciaddr:  [4]byte(p[12:16]),
		yiaddr:  [4]byte(p[16:20]),
		siaddr:  [4]byte(p[20:24]),
		giaddr:  [4]byte(p[24:28]),
		chaddr:  [16]byte(p[28:44]),
		sname:   [64]byte(p[44:108]),
		file:    [128]byte(p[108:236]),
		options: options,
	}, nil
}

func options(p []byte) ([]option, error) {
	options := make([]option, len(p))
	n := 0
	for i, code := range p {
		switch code {
		case SubnetMask:
			_, ipnet, err := net.ParseCIDR(rangeStart)
			if err != nil {
				return nil, err
			}
			options[i] = option{
				code:  code,
				len:   4,
				value: ipnet.Mask,
			}
			n++
		case Router:
			options[i] = option{
				code:  code,
				len:   4,
				value: net.ParseIP(defaultRouter)[12:16],
			}
			n++
		case DomainServer:
			options[i] = option{
				code:  code,
				len:   4,
				value: net.ParseIP(dns)[12:16],
			}
			n++
		case BroadcastAddress:
			_, ipnet, err := net.ParseCIDR(rangeStart)
			if err != nil {
				return nil, err
			}
			options[i] = option{
				code:  code,
				len:   4,
				value: []byte{ipnet.IP[0] | ^ipnet.Mask[0], ipnet.IP[1] | ^ipnet.Mask[1], ipnet.IP[2] | ^ipnet.Mask[2], ipnet.IP[3] | ^ipnet.Mask[3]},
			}
			n++
		}
	}
	return options[:n], nil
}

func (d dhcp) msgType() byte {
	t := byte(0)
	for _, option := range d.options {
		if option.code != DHCPMsgType {
			continue
		}
		t = option.value[0]
		break
	}
	return t
}

func (d dhcp) parameterList() []byte {
	var t []byte
	for _, option := range d.options {
		if option.code != ParameterList {
			continue
		}
		t = option.value
		break
	}
	return t
}

func (d dhcp) isPXE() bool {
	t := false
	for _, option := range d.options {
		if option.code != ClassId {
			continue
		}
		if bytes.Contains(option.value, []byte("PXEClient")) {
			t = true
		}
	}
	return t
}

func (d dhcp) write(p []byte) (int, error) {
	if len(p) < leastMessageLen {
		return 0, errors.New("buffer is too small")
	}

	p[0] = d.op
	p[1] = d.htype
	p[2] = d.hlen
	p[3] = d.hops
	copy(p[4:8], d.xid[:])
	copy(p[8:10], d.secs[:])
	copy(p[10:12], d.flags[:])
	copy(p[12:16], d.ciaddr[:])
	copy(p[16:20], d.yiaddr[:])
	copy(p[20:24], d.siaddr[:])
	copy(p[24:28], d.giaddr[:])
	copy(p[28:44], d.chaddr[:])
	copy(p[44:108], d.sname[:])
	copy(p[108:236], d.file[:])
	copy(p[236:240], []byte{99, 130, 83, 99})

	n := 240
	for _, option := range d.options {
		if len(p) <= n+2+int(option.len) {
			return 0, errors.New("buffer is too small")
		}
		p[n] = option.code
		p[n+1] = option.len
		copy(p[n+2:n+2+int(option.len)], option.value)
		n += 2 + int(option.len)
	}
	p[n] = End
	n++
	for n < leastMessageLen {
		p[n] = Pad
		n++
	}

	return n, nil
}

func (d leaseDB) pick(haddr [16]byte) ([4]byte, error) {
	addr, ok := d[string(haddr[:])]
	if !ok {
		iaddr, _, err := net.ParseCIDR(rangeStart)
		if err != nil {
			return [4]byte{}, err
		}
		iaddr = iaddr[12:16]
		iaddr[3] = iaddr[3] + current
		current++
		d[string(haddr[:])] = strconv.Itoa(int(iaddr[0])) + "." + strconv.Itoa(int(iaddr[1])) + "." + strconv.Itoa(int(iaddr[2])) + "." + strconv.Itoa(int(iaddr[3]))
		addr = d[string(haddr[:])]
	}
	picked := [4]byte{}
	copy(picked[:], net.ParseIP(addr)[12:16])
	return picked, nil
}
