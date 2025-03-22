package tao

import (
	"encoding/json"
	"flag"
	"io"
	"log/slog"
	"os"
	"time"

	tftp "github.com/callus-corn/tao/internal/tftp"
)

type config struct {
	TFTP tftpConfig `json:"TFTP"`
}

type tftpConfig struct {
	IsEnable bool   `json:"IsEnable"`
	Address  string `json:"Address"`
	SrvDir   string `json:"SrvDir"`
}

var logger *slog.Logger
var conf config

func Main() {
	logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := setup(); err != nil {
		logger.Error(err.Error(), "module", "TAO")
		os.Exit(1)
	}

	if err := tftp.Listen(conf.TFTP.Address, conf.TFTP.SrvDir); err != nil {
		logger.Error(err.Error(), "module", "TAO")
		os.Exit(1)
	}
	logger.Info("TFTP is listening at "+conf.TFTP.Address+conf.TFTP.SrvDir, "module", "TAO")

	for {
		logger.Info("TAO start successfully", "module", "TAO")
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
