package tftp

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
)

type TFTP struct {
	blocks  [][]byte
	blockNo int
}

const UdpMax = 65536
const blockMax = 65536
const blockLen = 512
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
	fmt.Println(conn.LocalAddr().String())
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

		if opc := udpBuffer[1]; opc != OpcRRQ {
			logger.Error("Illegal TFTP operation form "+addr.String(), "module", "TFTP")
			response := []byte{0, OpcERROR, 0, IllegalTFTPOperation, 0}
			conn.WriteTo(response, addr)
			continue
		}

		tftp, err := RRQ(udpBuffer)
		if err != nil {
			logger.Error(err.Error(), "module", "TFTP")
			response := []byte{0, OpcERROR, 0, FileNotFound, 0}
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

	block := make([]byte, blockLen)
	_, err := tftp.Read(block)
	if err != nil {
		logger.Error(err.Error(), "module", "TFTP")
		response := []byte{0, OpcERROR, 0, Accessviolation, 0}
		conn.WriteTo(response, addr)
		return
	}
	head := []byte{0, OpcDATA, byte(tftp.blockNo >> 8), byte(tftp.blockNo)}
	response := append(head, block...)
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

		block := make([]byte, blockLen)
		n, err := tftp.Read(block)
		if err != nil {
			logger.Error(err.Error(), "module", "TFTP")
			response := []byte{0, OpcERROR, 0, Accessviolation, 0}
			conn.WriteTo(response, addr)
			return
		}
		if n < blockLen {
			block = block[:n]
		}
		head := []byte{0, OpcDATA, byte(tftp.blockNo >> 8), byte(tftp.blockNo)}
		response := append(head, block...)
		conn.WriteTo(response, addr)
	}
}

func RRQ(p []byte) (*TFTP, error) {
	filename := bytes.Split(p[2:], []byte{0})[0]
	path := srvDir + "/" + string(filename)
	fp, err := os.Open(path)
	if err != nil {
		return nil, err
	}

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

	return &TFTP{blocks, 1}, nil
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

func (t *TFTP) Read(p []byte) (n int, err error) {
	block := t.blocks[t.blockNo-1]
	copy(p, block)
	return len(block), nil
}
