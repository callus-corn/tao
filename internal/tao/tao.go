package tao

import (
	"encoding/json"
	"flag"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/callus-corn/tao/internal/dhcp"
	"github.com/callus-corn/tao/internal/http"
	"github.com/callus-corn/tao/internal/tftp"
)

type config struct {
	TFTP tftp.TFTPConfig `json:"TFTP"`
	DHCP dhcp.DHCPConfig `json:"DHCP"`
	HTTP http.HTTPConfig `json:"HTTP"`
}

var logger *slog.Logger
var conf config

func Main() {
	logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := setup(); err != nil {
		logger.Error(err.Error(), "module", "TAO")
		os.Exit(1)
	}

	if err := tftp.Listen(conf.TFTP); err != nil {
		logger.Error(err.Error(), "module", "TAO")
		os.Exit(1)
	}
	logger.Info("TFTP is listening at "+conf.TFTP.Address+conf.TFTP.SrvDir, "module", "TAO")

	if err := dhcp.Listen(conf.DHCP); err != nil {
		logger.Error(err.Error(), "module", "TAO")
		os.Exit(1)
	}
	logger.Info("DHCP is listening at "+conf.DHCP.Address, "module", "TAO")

	if err := http.Listen(conf.HTTP); err != nil {
		logger.Error(err.Error(), "module", "TAO")
		os.Exit(1)
	}
	logger.Info("HTTP is listening at "+conf.HTTP.Address+conf.HTTP.SrvDir, "module", "TAO")

	logger.Info("TAO start successfully", "module", "TAO")

	for {
		time.Sleep(24 * time.Hour)
	}
}

func setup() error {
	fname := flag.String("conf", "/etc/tao/tao.conf", "config file")
	flag.Parse()

	jsonFile, err := os.Open(*fname)
	if err != nil {
		return err
	}
	defer jsonFile.Close()
	jsonData, err := io.ReadAll(jsonFile)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonData, &conf)
}
