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
	file    *os.File
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
const optBlocksize = "blksize"

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
		logger.Info("TFTP RRQ option", "module", "TFTP", "address", client.String(), "option", tftp.option)

		conn, err := net.ListenPacket("udp", host+":0")
		if err != nil {
			logger.Error(err.Error(), "module", "TFTP")
		}
		logger.Info("TFTP send file", "module", "TFTP", "address", client.String(), "filename", tftp.file.Name())

		go handleTFTP(conn, client, tftp)
	}
}

func handleTFTP(conn net.PacketConn, client net.Addr, tftp *tftp) {
	defer conn.Close()
	defer tftp.close()

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

func rrq(p []byte) (*tftp, error) {
	filename := string(bytes.Split(p[2:], []byte{0})[0])
	path := srvDir + "/" + filename
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	option := make(map[string]string)
	options := bytes.Split(p[2:], []byte{0})[2:]
	for i := 0; i < len(options); i += 2 {
		if len(options[i]) == 0 {
			continue
		}
		option[strings.ToLower(string(options[i]))] = string(options[i+1])
	}

	blocks := make([][]byte, blockMax)
	blockNo := 1
	if len(option) > 0 {
		blockNo = 0
	}
	tftp := &tftp{blocks, blockNo, file, option}
	if err = tftp.loadFile(); err != nil {
		return nil, err
	}
	return tftp, nil
}

func (t *tftp) ack(p []byte) error {
	if len(p) < 4 {
		return errors.New("invalid packet")
	}
	if p[1] != opcACK {
		return errors.New("opc is not ACK")
	}
	ack := (int(p[2]) << 8) + int(p[3])
	if ack != t.blockNo {
		return errors.New("invalid ACK number")
	}

	t.blockNo = ack + 1
	if t.blockNo >= blockMax {
		t.reloadFile()
		t.blockNo = 0
	}

	return nil
}

func (t *tftp) data() ([]byte, error) {
	head := []byte{0, opcDATA, byte(t.blockNo >> 8), byte(t.blockNo)}
	block := t.blocks[t.blockNo]
	return append(head, block...), nil
}

func newError(code byte) []byte {
	return []byte{0, opcERROR, 0, code, 0}
}

func (t *tftp) oack() ([]byte, error) {
	head := []byte{0, opcOACK}
	options := []byte{}
	for k, v := range t.option {
		switch k {
		case optBlocksize:
			options = append(options, []byte(k)...)
			options = append(options, 0)
			options = append(options, []byte(v)...)
			options = append(options, 0)
		}
	}
	return append(head, options...), nil
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

func (t *tftp) close() {
	t.file.Close()
}

func (t *tftp) blockSize() (int, error) {
	blockSize := 512
	if v, ok := t.option[optBlocksize]; ok {
		var err error
		blockSize, err = strconv.Atoi(v)
		if err != nil {
			return 0, err
		}
	}
	return blockSize, nil
}

func (t *tftp) loadFile() error {
	blockSize, err := t.blockSize()
	if err != nil {
		return err
	}

	for i := 1; i < len(t.blocks); i++ {
		block := make([]byte, blockSize)
		n, err := t.file.Read(block)
		if n < blockSize {
			t.blocks[i] = block[:n]
			break
		}
		if err != nil {
			return err
		}
		t.blocks[i] = block
	}
	return nil
}

func (t *tftp) reloadFile() error {
	blockSize, err := t.blockSize()
	if err != nil {
		return err
	}

	for i := range len(t.blocks) {
		block := make([]byte, blockSize)
		n, err := t.file.Read(block)
		if n < blockSize {
			t.blocks[i] = block[:n]
			break
		}
		if err != nil {
			return err
		}
		t.blocks[i] = block
	}
	return nil
}
