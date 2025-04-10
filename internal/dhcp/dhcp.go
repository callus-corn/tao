package dhcp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"slices"
)

type DHCPConfig struct {
	IsEnable bool   `json:"IsEnable"`
	Address  string `json:"Address"`
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
const End = 255

// not support
// const TimeOffset = 2
// const TimeServer = 4
// const NameServer = 5
// const Hostname = 12
// const BootFileSize = 13
// const DomanName = 15
// const RootPath = 17
// const ExtensionFile = 18
// const MaxDGAssembly = 22
// const DefaultIPTTL = 23
// const StaticRoute = 33
// const NISDomain = 40
// const NISServers = 41
// const NTPServers = 42
// const VendorSpecific = 43
// const AddressRequest = 50
// const Overload = 52
// const DHCPMaxMsgSize = 57
// const RenewalTime = 58
// const RebindingTime = 59
// const ClassId = 60
// const BootFileName = 67
// const UUID = 97
// const DHCPCaptivePortal = 114
// const DomainSearch = 119
// const SIPServersDHCPOption = 120
// const ClasslessStaticRouteOption = 121

// Sub options are not supported
// const PXEDISCOVERYCONTROL = 6
// const PXEBOOTSERVERS = 8
// const PXEBOOTMENU = 9
// const PXEBOOTITEM = 71

const DHCPDISCOVER = 1
const DHCPOFFER = 2
const DHCPREQUEST = 3
const DHCPDECLINE = 4
const DHCPACK = 5
const DHCPNAK = 6
const DHCPRELEASE = 7
const DHCPINFORM = 8

var logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
var address string

func Listen(conf DHCPConfig) error {
	address = conf.Address

	conn, err := net.ListenPacket("udp", address)
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

	switch dhcp.msgType() {
	case DHCPDISCOVER:
		logger.Info("receve DHCPDISCOVER", "module", "DHCP", "message", fmt.Sprintf("%v", dhcp))
		tx := make([]byte, udpMax)
		n, err := dhcp.offer(tx)
		if err != nil {
			logger.Error(err.Error(), "module", "DHCP")
			return
		}
		addr, err := net.ResolveUDPAddr("udp", "255.255.255.255:68")
		if err != nil {
			logger.Error(err.Error(), "module", "DHCP")
			return
		}
		conn.WriteTo(slices.Clone(tx[:n]), addr)
	case DHCPOFFER:
		logger.Info("receve DHCPOFFER", "module", "DHCP", "message", fmt.Sprintf("%v", dhcp))
	case DHCPREQUEST:
		logger.Info("receve DHCPREQUEST", "module", "DHCP", "message", fmt.Sprintf("%v", dhcp))
		tx := make([]byte, udpMax)
		n, err := dhcp.ack(tx)
		if err != nil {
			logger.Error(err.Error(), "module", "DHCP")
			return
		}
		addr, err := net.ResolveUDPAddr("udp", "255.255.255.255:68")
		if err != nil {
			logger.Error(err.Error(), "module", "DHCP")
			return
		}
		conn.WriteTo(slices.Clone(tx[:n]), addr)
	case DHCPDECLINE:
		logger.Info("receve DHCPDECLINE", "module", "DHCP", "message", fmt.Sprintf("%v", dhcp))
	case DHCPACK:
		logger.Info("receve DHCPACK", "module", "DHCP", "message", fmt.Sprintf("%v", dhcp))
	case DHCPNAK:
		logger.Info("receve DHCPNAK", "module", "DHCP", "message", fmt.Sprintf("%v", dhcp))
	case DHCPRELEASE:
		logger.Info("receve DHCPRELEASE", "module", "DHCP", "message", fmt.Sprintf("%v", dhcp))
	case DHCPINFORM:
		logger.Info("receve DHCPINFORM", "module", "DHCP", "message", fmt.Sprintf("%v", dhcp))
	default:
		logger.Info("receved message is not supported", "module", "DHCP", "message", fmt.Sprintf("%v", dhcp))
	}
}

func (d *dhcp) offer(p []byte) (int, error) {
	msgType := option{
		code:  DHCPMsgType,
		len:   1,
		value: []byte{DHCPOFFER},
	}

	dhcpServerId := option{
		code:  DHCPServerId,
		len:   4,
		value: []byte{10, 0, 1, 1},
	}

	t := make([]byte, 4)
	binary.BigEndian.PutUint32(t, 864000)
	addressTime := option{
		code:  AddressTime,
		len:   4,
		value: t,
	}

	o := options(d.parameterList())

	options := make([]option, 0, 3+len(o))
	options = append(options, msgType, dhcpServerId, addressTime)
	options = append(options, o...)

	fname := [128]byte{0}
	copy(fname[:], []byte("EFI/boot/bootx64.efi"))

	return dhcp{
		op:      BOOTREPLY,
		htype:   ETHERNET,
		hlen:    ETHERNETHLEN,
		hops:    0,
		xid:     d.xid,
		secs:    d.secs,
		flags:   d.flags,
		ciaddr:  [4]byte{0},
		yiaddr:  [4]byte{10, 0, 1, 2},
		siaddr:  [4]byte{10, 0, 1, 1},
		giaddr:  d.giaddr,
		chaddr:  d.chaddr,
		sname:   [64]byte{0},
		file:    fname,
		options: options,
	}.write(p)
}

func (d *dhcp) ack(p []byte) (int, error) {
	msgType := option{
		code:  DHCPMsgType,
		len:   1,
		value: []byte{DHCPACK},
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
		value: []byte{10, 0, 1, 1},
	}

	o := options(d.parameterList())

	options := make([]option, 0, 3+len(o))
	options = append(options, msgType, addressTime, dhcpServerId)
	options = append(options, o...)

	fname := [128]byte{0}
	copy(fname[:], []byte("EFI/boot/bootx64.efi"))

	return dhcp{
		op:      BOOTREPLY,
		htype:   ETHERNET,
		hlen:    ETHERNETHLEN,
		hops:    0,
		xid:     d.xid,
		secs:    [2]byte{0},
		flags:   d.flags,
		ciaddr:  [4]byte{0},
		yiaddr:  [4]byte{10, 0, 1, 2},
		siaddr:  [4]byte{10, 0, 1, 1},
		giaddr:  d.giaddr,
		chaddr:  d.chaddr,
		sname:   [64]byte{0},
		file:    fname,
		options: options,
	}.write(p)
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

func options(p []byte) []option {
	options := make([]option, len(p))
	for i, code := range p {
		switch code {
		case SubnetMask:
			options[i] = option{
				code:  code,
				len:   4,
				value: []byte{255, 0, 0, 0},
			}
		case Router:
			options[i] = option{
				code:  code,
				len:   4,
				value: []byte{10, 0, 0, 1},
			}
		case DomainServer:
			options[i] = option{
				code:  code,
				len:   4,
				value: []byte{8, 8, 8, 8},
			}
		case MTUInterface:
			options[i] = option{
				code:  code,
				len:   2,
				value: []byte{5, 220},
			}
		case BroadcastAddress:
			options[i] = option{
				code:  code,
				len:   4,
				value: []byte{10, 255, 255, 255},
			}
		}
	}
	return options
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
