package tftp

import (
	"bytes"
	"errors"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
)

type tftp struct {
	blocks  [][]byte
	blockNo int
	option  map[string]string
}

const udpMax = 65536
const blockMax = 65536
const opcRRQ = 1
const opcDATA = 3
const opcACK = 4
const opcERROR = 5
const opcOACK = 6
const fileNotFound = 1
const accessviolation = 2
const illegalTFTPOperation = 4
const requestHasBeenDeniend = 8

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
		_, client, err := conn.ReadFrom(udpBuffer)
		if err != nil {
			logger.Error(err.Error(), "module", "TFTP")
		}
		logger.Info("TFTP connection start", "module", "TFTP", "address", client.String())

		if err := isRRQ(udpBuffer); err != nil {
			logger.Error("Illegal TFTP operation form "+client.String(), "module", "TFTP")
			response := newError(illegalTFTPOperation)
			conn.WriteTo(response, client)
			continue
		}

		tftp, err := rrq(udpBuffer)
		if err != nil {
			logger.Error(err.Error(), "module", "TFTP")
			response := newError(fileNotFound)
			conn.WriteTo(response, client)
			continue
		}

		conn, err := net.ListenPacket("udp", host+":0")
		if err != nil {
			logger.Error(err.Error(), "module", "TFTP")
		}
		logger.Info("TFTP send file", "module", "TFTP", "address", client.String())

		go handleTFTP(conn, client, tftp)
	}
}

func handleTFTP(conn net.PacketConn, client net.Addr, tftp *tftp) {
	defer conn.Close()

	udpBuffer := make([]byte, udpMax)
	if len(tftp.option) > 0 {
		response, err := tftp.oack()
		if err != nil {
			logger.Error(err.Error(), "module", "TFTP")
			response := newError(requestHasBeenDeniend)
			conn.WriteTo(response, client)
			return
		}
		conn.WriteTo(response, client)

		for {
			_, ackClient, err := conn.ReadFrom(udpBuffer)
			if err != nil {
				logger.Error(err.Error(), "module", "TFTP")
				continue
			}
			if err = isClient(ackClient, client); err != nil {
				continue
			}
			if err = tftp.ack(udpBuffer); err != nil {
				logger.Error(err.Error(), "module", "TFTP")
				continue
			}
			break
		}
	}

	response, err := tftp.data()
	if err != nil {
		logger.Error(err.Error(), "module", "TFTP")
		response := newError(accessviolation)
		conn.WriteTo(response, client)
		return
	}
	conn.WriteTo(response, client)

	for {
		_, ackClient, err := conn.ReadFrom(udpBuffer)
		if err != nil {
			logger.Error(err.Error(), "module", "TFTP")
			continue
		}
		if err = isClient(ackClient, client); err != nil {
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
			conn.WriteTo(response, client)
			return
		}
		conn.WriteTo(response, client)
	}
}

func isClient(a net.Addr, b net.Addr) error {
	if a.String() != b.String() {
		return errors.New("invalid client")
	}
	return nil
}

func isRRQ(p []byte) error {
	if len(p) < 2 {
		return errors.New("invalid packet")
	}
	if p[1] != opcRRQ {
		return errors.New("opc is not RRQ")
	}
	return nil
}

func rrq(p []byte) (*tftp, error) {
	filename := string(bytes.Split(p[2:], []byte{0})[0])
	path := srvDir + "/" + filename
	fp, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	option := make(map[string]string)
	options := bytes.Split(p[2:], []byte{0})
	for i := range options {
		if strings.ToLower(string(options[i])) == "blksize" {
			option["blksize"] = string(options[i+1])
		}
	}

	blockNo := 1
	if len(option) > 0 {
		blockNo = 0
	}

	blockSize := 512
	for k, v := range option {
		if k == "blksize" {
			blockSize, err = strconv.Atoi(v)
			if err != nil {
				return nil, err
			}
		}
	}

	blocks := make([][]byte, blockMax)
	for i := range blockMax {
		block := make([]byte, blockSize)
		n, err := fp.Read(block)
		if n < blockSize {
			blocks[i] = block[:n]
			break
		}
		if err != nil {
			return nil, err
		}
		blocks[i] = block
	}

	return &tftp{blocks, blockNo, option}, nil
}

func (t *tftp) ack(p []byte) error {
	if len(p) < 4 {
		return errors.New("invalid packet")
	}
	if p[1] != opcACK {
		return errors.New("opc is not ACK")
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

func (t *tftp) oack() ([]byte, error) {
	head := []byte{0, opcOACK}
	opt := []byte("blksize")
	blksize := []byte(t.option["blksize"])
	r := append(head, opt...)
	r = append(r, 0)
	r = append(r, blksize...)
	r = append(r, 0)
	return r, nil
}
