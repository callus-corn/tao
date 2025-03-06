package tftp

import (
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type testCase struct {
	name     string
	bytes    int
	blockLen int
}

const fname = "tmp.bin"

func TestTFTP(t *testing.T) {
	tests := []testCase{
		{name: "0 byte file", bytes: 0, blockLen: 512},
		{name: "1 byte file", bytes: 1, blockLen: 512},
		{name: "10 byte file", bytes: 10, blockLen: 512},
		{name: "100 byte file", bytes: 100, blockLen: 512},
		{name: "1k byte file", bytes: 1000, blockLen: 512},
		{name: "10k byte file", bytes: 10 * 1000, blockLen: 512},
		{name: "100k byte file", bytes: 100 * 1000, blockLen: 512},
		{name: "1m byte file", bytes: 1000 * 1000, blockLen: 512},
		{name: "10m byte file", bytes: 10 * 1000 * 1000, blockLen: 512},
		{name: "511 byte file", bytes: 511, blockLen: 512},
		{name: "512 byte file", bytes: 512, blockLen: 512},
		{name: "513 byte file", bytes: 513, blockLen: 512},
		{name: "1023 byte file", bytes: 1023, blockLen: 512},
		{name: "1024 byte file", bytes: 1024, blockLen: 512},
		{name: "1025 byte file", bytes: 1025, blockLen: 512},
	}

	for _, tc := range tests {
		var wants []byte
		for _ = range tc.bytes {
			wants = append(wants, byte(rand.Int()))
		}

		func() {
			dir := t.TempDir()
			path := filepath.Join(dir, fname)
			srvDir = dir

			err := os.WriteFile(path, wants, 0644)
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(path)

			got, err := getFile(tc)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(wants, got) {
				t.Fatalf("expected %v, got: %v", wants, got)
			}
		}()
	}
}

func getFile(tc testCase) ([]byte, error) {
	blockLen := tc.blockLen

	head := []byte{0, opcRRQ}
	req := append(head, []byte(fname)...)
	tftp, err := rrq(req)
	if err != nil {
		return nil, err
	}

	var got []byte
	i := 1
	for {
		block, err := tftp.data()
		if err != nil {
			return nil, err
		}
		got = append(got, block[4:]...)

		if len(block) < blockLen {
			break
		}
		tftp.ack([]byte{0, opcACK, byte(i >> 8), byte(i)})
		i++
	}

	return got, nil
}
