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

type TFTPConfig struct {
	IsEnable bool   `json:"IsEnable"`
	Address  string `json:"Address"`
	SrvDir   string `json:"SrvDir"`
}

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
const optTransfersize = "tsize"

var logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
var host string
var srvDir = "./"

func Listen(conf TFTPConfig) error {
	address := conf.Address
	srvDir = conf.SrvDir

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

	rx := make([]byte, udpMax)
	for {
		_, client, err := conn.ReadFrom(rx)
		if err != nil {
			logger.Error(err.Error(), "module", "TFTP")
			continue
		}
		logger.Info("TFTP connection start", "module", "TFTP", "address", client.String())

		if err := isERROR(rx); err != nil {
			logger.Error(err.Error(), "module", "TFTP")
			continue
		}

		if err := isRRQ(rx); err != nil {
			logger.Error("Illegal TFTP operation form "+client.String(), "module", "TFTP")
			response := newError(illegalTFTPOperation)
			conn.WriteTo(response, client)
			continue
		}

		tftp, err := rrq(rx)
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
			continue
		}
		logger.Info("TFTP send file", "module", "TFTP", "address", client.String(), "filename", tftp.file.Name())

		go handleTFTP(conn, client, tftp)
	}
}

func handleTFTP(conn net.PacketConn, client net.Addr, tftp *tftp) {
	defer conn.Close()
	defer tftp.close()

	rx := make([]byte, udpMax)
	tx := make([]byte, udpMax)
	if len(tftp.option) > 0 {
		//if false {
		n, err := tftp.oack(tx)
		if err != nil {
			logger.Error(err.Error(), "module", "TFTP")
			response := newError(requestHasBeenDeniend)
			conn.WriteTo(response, client)
			return
		}
		oack := tx[:n]
		conn.WriteTo(oack, client)

		for {
			_, ackClient, err := conn.ReadFrom(rx)
			if err != nil {
				logger.Error(err.Error(), "module", "TFTP")
				continue
			}
			if err = isClient(ackClient, client); err != nil {
				continue
			}
			if err := isERROR(rx); err != nil {
				logger.Error(err.Error(), "module", "TFTP")
				return
			}
			if err = tftp.ack(rx); err != nil {
				logger.Error(err.Error(), "module", "TFTP")
				continue
			}
			break
		}
	}

	n, err := tftp.data(tx)
	if err != nil {
		logger.Error(err.Error(), "module", "TFTP")
		response := newError(accessviolation)
		conn.WriteTo(response, client)
		return
	}
	data := tx[:n]
	conn.WriteTo(data, client)

	for {
		_, ackClient, err := conn.ReadFrom(rx)
		if err != nil {
			logger.Error(err.Error(), "module", "TFTP")
			continue
		}
		if err = isClient(ackClient, client); err != nil {
			continue
		}
		if err := isERROR(rx); err != nil {
			logger.Error(err.Error(), "module", "TFTP")
			return
		}
		if err = tftp.ack(rx); err != nil {
			logger.Error(err.Error(), "module", "TFTP")
			continue
		}

		n, err := tftp.data(tx)
		if err != nil {
			logger.Error(err.Error(), "module", "TFTP")
			response := newError(accessviolation)
			conn.WriteTo(response, client)
			return
		}
		data := tx[:n]
		conn.WriteTo(data, client)
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

	blocks := make([][]byte, 1)
	blockNo := 1
	if len(option) > 0 {
		blockNo = 0
	}
	tftp := &tftp{blocks, blockNo, file, option}
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
		t.blockNo = 0
	}

	return nil
}

func (t *tftp) data(p []byte) (int, error) {
	t.loadFile()
	head := []byte{0, opcDATA, byte(t.blockNo >> 8), byte(t.blockNo)}
	for i := range len(head) {
		p[i] = head[i]
	}
	for i := range len(t.blocks[0]) {
		p[len(head)+i] = t.blocks[0][i]
	}
	return len(head) + len(t.blocks[0]), nil
}

func newError(code byte) []byte {
	return []byte{0, opcERROR, 0, code, 0}
}

func (t *tftp) oack(p []byte) (int, error) {
	head := []byte{0, opcOACK}
	options := []byte{}
	for k, v := range t.option {
		switch k {
		case optBlocksize:
			options = append(options, []byte(k)...)
			options = append(options, 0)
			options = append(options, []byte(v)...)
			options = append(options, 0)
		case optTransfersize:
			info, err := t.file.Stat()
			if err != nil {
				return 0, err
			}
			tsize := strconv.Itoa(int(info.Size()))
			options = append(options, []byte(k)...)
			options = append(options, 0)
			options = append(options, tsize...)
			options = append(options, 0)
		}
	}
	for i := range len(head) {
		p[i] = head[i]
	}
	for i := range len(options) {
		p[len(head)+i] = options[i]
	}
	return len(head) + len(options), nil
}

func isClient(a net.Addr, b net.Addr) error {
	if a.String() != b.String() {
		return errors.New("invalid client")
	}
	return nil
}

func isERROR(p []byte) error {
	if len(p) < 2 {
		return errors.New("invalid packet")
	}
	if p[1] == opcERROR {
		return errors.New("error code " + string(p[3]) + " " + string(bytes.Split(p[4:], []byte{0})[0]))
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

	for i := range len(t.blocks) {
		if t.blocks[i] == nil {
			block := make([]byte, blockSize)
			t.blocks[i] = block
		}
		n, err := t.file.Read(t.blocks[i])
		if n < blockSize {
			t.blocks[i] = t.blocks[i][:n]
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}
