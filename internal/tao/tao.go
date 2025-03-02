package tao

import (
	"log/slog"
	"os"
	"time"

	tftp "github.com/callus-corn/tao/internal/tftp"
)

func Main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	address := "192.168.1.101:69"
	if err := tftp.Listen(address); err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	logger.Info("TFTP is listening at "+address, "module", "TAO")

	for {
		logger.Info("TAO start successfully", "module", "TAO")
		time.Sleep(24 * time.Hour)
	}
}
