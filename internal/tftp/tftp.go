package tftp

import (
	"bytes"
	"errors"
	"log/slog"
	"net"
	"os"
)

type tftp struct {
	blocks  [][]byte
	blockNo int
}

const udpMax = 65536
const blockMax = 65536
const opcRRQ = 1
const opcDATA = 3
const opcACK = 4
const opcERROR = 5
const fileNotFound = 1
const accessviolation = 2
const illegalTFTPOperation = 4

var logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
var host string
var srvDir = "/home/ubuntu"

func Listen(address string) error {
	var err error = nil
	host, _, err = net.SplitHostPort(address)
	if err != nil {
		return err
	}

	conn, err := net.ListenPacket("udp", address)
	if err != nil {
		return err
	}
	go listen(conn)

	return nil
}

func listen(conn net.PacketConn) {
	defer conn.Close()

	udpBuffer := make([]byte, udpMax)
	for {
		_, addr, err := conn.ReadFrom(udpBuffer)
		if err != nil {
			logger.Error(err.Error(), "module", "TFTP")
		}
		logger.Info("TFTP connection start", "module", "TFTP", "address", addr.String())

		if err := checkRRQ(udpBuffer); err != nil {
			logger.Error("Illegal TFTP operation form "+addr.String(), "module", "TFTP")
			response := newError(illegalTFTPOperation)
			conn.WriteTo(response, addr)
			continue
		}

		tftp, err := rrq(udpBuffer)
		if err != nil {
			logger.Error(err.Error(), "module", "TFTP")
			response := newError(fileNotFound)
			conn.WriteTo(response, addr)
			continue
		}

		conn, err := net.ListenPacket("udp", host+":0")
		if err != nil {
			logger.Error(err.Error(), "module", "TFTP")
		}
		logger.Info("TFTP send file", "module", "TFTP", "address", addr.String())

		go handleTFTP(conn, addr, tftp)
	}
}

func handleTFTP(conn net.PacketConn, addr net.Addr, tftp *tftp) {
	defer conn.Close()

	response, err := tftp.data()
	if err != nil {
		logger.Error(err.Error(), "module", "TFTP")
		response := newError(accessviolation)
		conn.WriteTo(response, addr)
		return
	}
	conn.WriteTo(response, addr)

	udpBuffer := make([]byte, udpMax)
	for {
		_, ackAddr, err := conn.ReadFrom(udpBuffer)
		if err != nil {
			logger.Error(err.Error(), "module", "TFTP")
			continue
		}
		if err = checkAddr(ackAddr, addr); err != nil {
			continue
		}
		if err = tftp.ack(udpBuffer); err != nil {
			logger.Error(err.Error(), "module", "TFTP")
			continue
		}

		response, err := tftp.data()
		if err != nil {
			logger.Error(err.Error(), "module", "TFTP")
			response := newError(accessviolation)
			conn.WriteTo(response, addr)
			return
		}
		conn.WriteTo(response, addr)
	}
}

func checkAddr(a net.Addr, b net.Addr) error {
	if a.String() != b.String() {
		return errors.New("invalid client")
	}
	return nil
}

func checkRRQ(p []byte) error {
	if len(p) < 2 {
		return errors.New("invalid packet")
	}
	if p[1] != opcRRQ {
		return errors.New("opc is not RRQ")
	}
	return nil
}

func rrq(p []byte) (*tftp, error) {
	filename := bytes.Split(p[2:], []byte{0})[0]
	path := srvDir + "/" + string(filename)
	fp, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	blockLen := 512

	blocks := make([][]byte, blockMax)
	for i := range blockMax {
		block := make([]byte, blockLen)
		n, err := fp.Read(block)
		if n < blockLen {
			blocks[i] = block[:n]
			break
		}
		if err != nil {
			return nil, err
		}
		blocks[i] = block
	}

	return &tftp{blocks, 1}, nil
}

func (t *tftp) ack(p []byte) error {
	if len(p) < 4 {
		return errors.New("invalid packet")
	}
	if p[1] != opcACK {
		return errors.New("opc is not RRQ")
	}

	ack := (int(p[2]) << 8) + int(p[3])
	if ack == t.blockNo {
		t.blockNo = ack + 1
	} else {
		return errors.New("invalid ACK number")
	}
	return nil
}

func (t *tftp) data() ([]byte, error) {
	block := t.blocks[t.blockNo-1]
	head := []byte{0, opcDATA, byte(t.blockNo >> 8), byte(t.blockNo)}
	data := append(head, block...)
	return data, nil
}

func newError(code byte) []byte {
	return []byte{0, opcERROR, 0, code, 0}
}
