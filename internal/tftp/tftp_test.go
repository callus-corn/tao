package tftp

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type testCase struct {
	name      string
	bytes     int
	blockSize int
}

const fname = "tmp.bin"

func TestTFTP(t *testing.T) {
	tests := []testCase{
		{name: "0 byte file, blockSize 512", bytes: 0, blockSize: 512},
		{name: "1 byte file, blockSize 512", bytes: 1, blockSize: 512},
		{name: "10 byte file, blockSize 512", bytes: 10, blockSize: 512},
		{name: "100 byte file, blockSize 512", bytes: 100, blockSize: 512},
		{name: "1k byte file, blockSize 512", bytes: 1000, blockSize: 512},
		{name: "10k byte file, blockSize 512", bytes: 10 * 1000, blockSize: 512},
		{name: "100k byte file, blockSize 512", bytes: 100 * 1000, blockSize: 512},
		{name: "1m byte file, blockSize 512", bytes: 1000 * 1000, blockSize: 512},
		{name: "10m byte file, blockSize 512", bytes: 10 * 1000 * 1000, blockSize: 512},
		{name: "511 byte file, blockSize 512", bytes: 511, blockSize: 512},
		{name: "512 byte file, blockSize 512", bytes: 512, blockSize: 512},
		{name: "513 byte file, blockSize 512", bytes: 513, blockSize: 512},
		{name: "1023 byte file, blockSize 512", bytes: 1023, blockSize: 512},
		{name: "1024 byte file, blockSize 512", bytes: 1024, blockSize: 512},
		{name: "1025 byte file, blockSize 512", bytes: 1025, blockSize: 512},
	}

	for _, tc := range tests {
		var wants []byte
		for range tc.bytes {
			wants = append(wants, byte(rand.Int()))
		}

		func() {
			println(tc.name)
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

func TestBlksize(t *testing.T) {
	tests := []testCase{
		{name: "0 byte file, blockSize 1468", bytes: 0, blockSize: 1468},
		{name: "1 byte file, blockSize 1468", bytes: 1, blockSize: 1468},
		{name: "10 byte file, blockSize 1468", bytes: 10, blockSize: 1468},
		{name: "100 byte file, blockSize 1468", bytes: 100, blockSize: 1468},
		{name: "1k byte file, blockSize 1468", bytes: 1000, blockSize: 1468},
		{name: "10k byte file, blockSize 1468", bytes: 10 * 1000, blockSize: 1468},
		{name: "100k byte file, blockSize 1468", bytes: 100 * 1000, blockSize: 1468},
		{name: "1m byte file, blockSize 1468", bytes: 1000 * 1000, blockSize: 1468},
		{name: "10m byte file, blockSize 1468", bytes: 10 * 1000 * 1000, blockSize: 1468},
		{name: "1467 byte file, blockSize 1468", bytes: 1467, blockSize: 1468},
		{name: "1468 byte file, blockSize 1468", bytes: 1468, blockSize: 1468},
		{name: "1469 byte file, blockSize 1468", bytes: 1469, blockSize: 1468},
		{name: "1468*2-1 byte file, blockSize 1468", bytes: 1468*2 - 1, blockSize: 1468},
		{name: "1468*2 byte file, blockSize 1468", bytes: 1468 * 2, blockSize: 1468},
		{name: "1468*2+1 byte file, blockSize 1468", bytes: 1468*2 + 1, blockSize: 1468},
	}

	for _, tc := range tests {
		println(tc.name)
		var wants []byte
		for range tc.bytes {
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
	blockSize := tc.blockSize

	head := []byte{0, opRRQ}
	option := append([]byte("blksize"), 0)
	option = append(option, fmt.Append(nil, blockSize)...)
	req := append(head, []byte(fname)...)
	req = append(req, 0)
	req = append(req, option...)
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

		if len(block) < blockSize {
			break
		}
		tftp.ack([]byte{0, opACK, byte(i >> 8), byte(i)})
		i++
	}

	return got, nil
}
