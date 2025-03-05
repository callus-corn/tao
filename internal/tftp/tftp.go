package tftp

import (
	"bytes"
	"errors"
	"log/slog"
	"net"
	"os"
)

type TFTP struct {
	blocks   [][]byte
	blockNo  int
	blockLen int
}

const UdpMax = 65536
const blockMax = 65536
const OpcRRQ = 1
const OpcDATA = 3
const OpcERROR = 5
const FileNotFound = 1
const Accessviolation = 2
const IllegalTFTPOperation = 4

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

	udpBuffer := make([]byte, UdpMax)
	for {
		_, addr, err := conn.ReadFrom(udpBuffer)
		if err != nil {
			logger.Error(err.Error(), "module", "TFTP")
		}
		logger.Info("TFTP connection start", "module", "TFTP", "address", addr.String())

		if err := checkRRQ(udpBuffer); err != nil {
			logger.Error("Illegal TFTP operation form "+addr.String(), "module", "TFTP")
			response := ERROR(IllegalTFTPOperation)
			conn.WriteTo(response, addr)
			continue
		}

		tftp, err := RRQ(udpBuffer)
		if err != nil {
			logger.Error(err.Error(), "module", "TFTP")
			response := ERROR(FileNotFound)
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

func handleTFTP(conn net.PacketConn, addr net.Addr, tftp *TFTP) {
	defer conn.Close()

	response, err := tftp.DATA()
	if err != nil {
		logger.Error(err.Error(), "module", "TFTP")
		response := ERROR(Accessviolation)
		conn.WriteTo(response, addr)
		return
	}
	conn.WriteTo(response, addr)

	udpBuffer := make([]byte, UdpMax)
	for {
		_, ackAddr, err := conn.ReadFrom(udpBuffer)
		if err != nil {
			logger.Error(err.Error(), "module", "TFTP")
			continue
		}
		if ackAddr.String() != addr.String() {
			continue
		}
		if err = tftp.ACK(udpBuffer); err != nil {
			logger.Error(err.Error(), "module", "TFTP")
			continue
		}

		response, err := tftp.DATA()
		if err != nil {
			logger.Error(err.Error(), "module", "TFTP")
			response := ERROR(Accessviolation)
			conn.WriteTo(response, addr)
			return
		}
		conn.WriteTo(response, addr)
	}
}

func checkRRQ(p []byte) error {
	if p[1] != OpcRRQ {
		return errors.New("opc is not RRQ")
	}
	return nil
}

func RRQ(p []byte) (*TFTP, error) {
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

	return &TFTP{blocks, 1, blockLen}, nil
}

func (t *TFTP) ACK(p []byte) error {
	ack := (int(p[2]) << 8) + int(p[3])
	if ack == t.blockNo {
		t.blockNo = ack + 1
	} else {
		return errors.New("invalid ACK number")
	}
	return nil
}

func (t *TFTP) DATA() ([]byte, error) {
	block := make([]byte, t.blockLen)
	n, err := t.Read(block)
	if err != nil {
		return nil, err
	}
	if n < t.blockLen {
		block = block[:n]
	}
	head := []byte{0, OpcDATA, byte(t.blockNo >> 8), byte(t.blockNo)}
	data := append(head, block...)
	return data, nil
}

func (t *TFTP) Read(p []byte) (n int, err error) {
	block := t.blocks[t.blockNo-1]
	copy(p, block)
	return len(block), nil
}

func ERROR(code byte) []byte {
	return []byte{0, OpcERROR, 0, code, 0}
}
