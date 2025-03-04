package tftp

import (
	"bytes"
	"log/slog"
	"net"
	"os"
	"strings"
)

type dataConn struct {
	conn net.PacketConn
	addr net.Addr
	file *os.File
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
	host = strings.ReplaceAll(address, ":69", "")
	conn, err := net.ListenPacket("udp", address)
	if err != nil {
		return err
	}
	go func(conn net.PacketConn) {
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

			filename := bytes.Split(udpBuffer[2:], []byte{0})[0]
			path := srvDir + "/" + string(filename)
			fp, err := os.Open(path)
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
			logger.Info("TFTP send file", "module", "TFTP", "address", addr.String(), "file", path)

			go dataConn{conn, addr, fp}.write()
		}
	}(conn)

	return nil
}

func (d dataConn) write() {
	defer d.conn.Close()
	defer d.file.Close()

	blockLen := 512
	blocks, err := d.readFileAll(blockLen)
	if err != nil {
		logger.Error(err.Error(), "module", "TFTP")
		response := []byte{0, OpcERROR, 0, Accessviolation, 0}
		d.conn.WriteTo(response, d.addr)
		return
	}

	head := []byte{0, OpcDATA, 0, 1}
	response := append(head, blocks[0]...)
	d.conn.WriteTo(response, d.addr)

	udpBuffer := make([]byte, UdpMax)
	for {
		_, addr, err := d.conn.ReadFrom(udpBuffer)
		if err != nil {
			logger.Error(err.Error(), "module", "TFTP")
			continue
		}
		if addr.String() != d.addr.String() {
			continue
		}

		ack := (int(udpBuffer[2]) << 8) + int(udpBuffer[3])
		blockNo := ack + 1
		head := []byte{0, OpcDATA, byte(blockNo >> 8), byte(blockNo)}
		response := append(head, blocks[blockNo-1]...)
		d.conn.WriteTo(response, d.addr)
	}
}

func (d dataConn) readFileAll(blockLen int) ([][]byte, error) {
	buffer := make([][]byte, blockMax)
	for i := range blockMax {
		fileBuffer := make([]byte, blockLen)
		n, err := d.file.Read(fileBuffer)
		if n < blockLen {
			buffer[i] = fileBuffer[:n]
			break
		}
		if err != nil {
			return nil, err
		}
		buffer[i] = fileBuffer
	}
	return buffer, nil
}
