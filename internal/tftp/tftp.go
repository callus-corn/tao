package tftp

import (
	"log/slog"
	"net"
	"os"
	"strings"
)

func Listen(address string) error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	conn, err := net.ListenPacket("udp", address)
	if err != nil {
		return err
	}
	go func(conn net.PacketConn) {
		defer conn.Close()

		mtu := 65536
		buffer := make([]byte, mtu)
		for {
			_, addr, err := conn.ReadFrom(buffer)
			if err != nil {
				logger.Error(err.Error(), "module", "TFTP")
			}
			logger.Info("TFTP connection start", "module", "TFTP", "address", addr.String())
			dataLen := 512

			if buffer[1] == 1 {
				host := strings.Split(address, ":")[0]
				dataconn, err := net.ListenPacket("udp", host+":0")
				if err != nil {
					logger.Error(err.Error(), "module", "TFTP")
				}

				filename := ""
				for _, v := range buffer[2:] {
					if v == 0 {
						break
					}
					filename = filename + string(v)
				}
				filepath := "/home/ubuntu/" + filename
				fileBuffer, err := os.ReadFile(filepath)
				if err != nil {
					logger.Error(err.Error(), "module", "TFTP")
					response := []byte{0, 5, 0, 1, 0}
					conn.WriteTo(response, addr)
					continue
				}
				logger.Info("TFTP send file", "module", "TFTP", "address", addr.String(), "file", filepath)

				fileLen := len(fileBuffer)
				var data []byte = nil
				if fileLen < dataLen {
					data = fileBuffer[:]
				} else {
					data = fileBuffer[:dataLen]
				}
				head := []byte{0, 3, 0, 1}
				response := append(head, data...)
				dataconn.WriteTo(response[:], addr)
				go func() {
					defer dataconn.Close()
					ack := make([]byte, mtu)
					for {
						_, addr, err := dataconn.ReadFrom(ack)
						if err != nil {
							logger.Error(err.Error(), "module", "TFTP")
						}

						var data []byte = nil
						ackNo := (int(ack[2]) << 8) + int(ack[3])
						if fileLen < ackNo*dataLen {
							break
						} else if (ackNo+1)*dataLen < fileLen {
							data = fileBuffer[ackNo*dataLen : (ackNo+1)*dataLen]
						} else {
							data = fileBuffer[ackNo*dataLen:]
						}
						dataNo := ackNo + 1
						head := []byte{0, 3, byte(dataNo >> 8), byte(dataNo)}
						response := append(head, data...)
						dataconn.WriteTo(response[:], addr)
					}
				}()
			} else {
				logger.Error("Illegal TFTP operation form "+addr.String(), "module", "TFTP")
				errPacket := [5]byte{0, 5, 0, 4, 0}
				conn.WriteTo(errPacket[:], addr)
			}
		}
	}(conn)

	return nil
}
