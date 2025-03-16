package tftp

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
)

type testCase struct {
	name   string
	bytes  int
	option map[string]string
}

const fname = "tmp.bin"

func TestTFTP(t *testing.T) {
	tests := []testCase{
		{name: "0    byte file", bytes: 0, option: nil},
		{name: "1    byte file", bytes: 1, option: nil},
		{name: "10   byte file", bytes: 10, option: nil},
		{name: "100  byte file", bytes: 100, option: nil},
		{name: "1k   byte file", bytes: 1000, option: nil},
		{name: "10k  byte file", bytes: 10 * 1000, option: nil},
		{name: "100k byte file", bytes: 100 * 1000, option: nil},
		{name: "1m   byte file", bytes: 1000 * 1000, option: nil},
		{name: "10m  byte file", bytes: 10 * 1000 * 1000, option: nil},
		{name: "100m byte file", bytes: 100 * 1000 * 1000, option: nil},
		{name: "1g   byte file", bytes: 1000 * 1000 * 1000, option: nil},
		{name: "512-1   byte file", bytes: 512 - 1, option: nil},
		{name: "512     byte file", bytes: 512, option: nil},
		{name: "512+1   byte file", bytes: 512 + 1, option: nil},
		{name: "512*2-1 byte file", bytes: 512*2 - 1, option: nil},
		{name: "512*2   byte file", bytes: 512 * 2, option: nil},
		{name: "512*2+1 byte file", bytes: 512*2 + 1, option: nil},
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
				t.Fatal("Fail at " + tc.name)
			}
		}()
	}
}

func TestBlksize(t *testing.T) {
	tests := []testCase{
		{name: "0 byte file, blockSize 1468", bytes: 0, option: map[string]string{"blksize": "1468"}},
		{name: "1 byte file, blockSize 1468", bytes: 1, option: map[string]string{"blksize": "1468"}},
		{name: "10 byte file, blockSize 1468", bytes: 10, option: map[string]string{"blksize": "1468"}},
		{name: "100 byte file, blockSize 1468", bytes: 100, option: map[string]string{"blksize": "1468"}},
		{name: "1k byte file, blockSize 1468", bytes: 1000, option: map[string]string{"blksize": "1468"}},
		{name: "10k byte file, blockSize 1468", bytes: 10 * 1000, option: map[string]string{"blksize": "1468"}},
		{name: "100k byte file, blockSize 1468", bytes: 100 * 1000, option: map[string]string{"blksize": "1468"}},
		{name: "1m byte file, blockSize 1468", bytes: 1000 * 1000, option: map[string]string{"blksize": "1468"}},
		{name: "10m byte file, blockSize 1468", bytes: 10 * 1000 * 1000, option: map[string]string{"blksize": "1468"}},
		{name: "100m byte file, blockSize 1468", bytes: 100 * 1000 * 1000, option: map[string]string{"blksize": "1468"}},
		{name: "1g byte file, blockSize 1468", bytes: 1000 * 1000 * 1000, option: map[string]string{"blksize": "1468"}},
		{name: "1468-1 byte file, blockSize 1468", bytes: 1468 - 1, option: map[string]string{"blksize": "1468"}},
		{name: "1468 byte file, blockSize 1468", bytes: 1468, option: map[string]string{"blksize": "1468"}},
		{name: "1468+1 byte file, blockSize 1468", bytes: 1468 + 1, option: map[string]string{"blksize": "1468"}},
		{name: "1468*2-1 byte file, blockSize 1468", bytes: 1468*2 - 1, option: map[string]string{"blksize": "1468"}},
		{name: "1468*2 byte file, blockSize 1468", bytes: 1468 * 2, option: map[string]string{"blksize": "1468"}},
		{name: "1468*2+1 byte file, blockSize 1468", bytes: 1468*2 + 1, option: map[string]string{"blksize": "1468"}},
		{name: "0 byte file, blockSize 8192", bytes: 0, option: map[string]string{"blksize": "8192"}},
		{name: "1 byte file, blockSize 8192", bytes: 1, option: map[string]string{"blksize": "8192"}},
		{name: "10 byte file, blockSize 8192", bytes: 10, option: map[string]string{"blksize": "8192"}},
		{name: "100 byte file, blockSize 8192", bytes: 100, option: map[string]string{"blksize": "8192"}},
		{name: "1k byte file, blockSize 8192", bytes: 1000, option: map[string]string{"blksize": "8192"}},
		{name: "10k byte file, blockSize 8192", bytes: 10 * 1000, option: map[string]string{"blksize": "8192"}},
		{name: "100k byte file, blockSize 8192", bytes: 100 * 1000, option: map[string]string{"blksize": "8192"}},
		{name: "1m byte file, blockSize 8192", bytes: 1000 * 1000, option: map[string]string{"blksize": "8192"}},
		{name: "10m byte file, blockSize 8192", bytes: 10 * 1000 * 1000, option: map[string]string{"blksize": "8192"}},
		{name: "100m byte file, blockSize 8192", bytes: 100 * 1000 * 1000, option: map[string]string{"blksize": "8192"}},
		{name: "1g byte file, blockSize 8192", bytes: 1000 * 1000 * 1000, option: map[string]string{"blksize": "8192"}},
		{name: "8192-1 byte file, blockSize 8192", bytes: 8192 - 1, option: map[string]string{"blksize": "8192"}},
		{name: "8192 byte file, blockSize 8192", bytes: 8192, option: map[string]string{"blksize": "8192"}},
		{name: "8192+1 byte file, blockSize 8192", bytes: 8192 + 1, option: map[string]string{"blksize": "8192"}},
		{name: "8192*2-1 byte file, blockSize 8192", bytes: 8192*2 - 1, option: map[string]string{"blksize": "8192"}},
		{name: "8192*2 byte file, blockSize 8192", bytes: 8192 * 2, option: map[string]string{"blksize": "8192"}},
		{name: "8192*2+1 byte file, blockSize 8192", bytes: 8192*2 + 1, option: map[string]string{"blksize": "8192"}},
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
				t.Fatal("Fail at " + tc.name)
			}
		}()
	}
}

func getFile(tc testCase) ([]byte, error) {
	var err error
	blockSize := 512

	head := []byte{0, opcRRQ}
	req := append(head, []byte(fname)...)
	req = append(req, 0)
	if tc.option != nil {
		option := append([]byte("blksize"), 0)
		option = append(option, fmt.Append(nil, tc.option["blksize"])...)
		req = append(req, option...)
		blockSize, err = strconv.Atoi(tc.option["blksize"])
		if err != nil {
			return nil, err
		}
	}
	tftp, err := rrq(req)
	if err != nil {
		return nil, err
	}

	if tc.option != nil {
		tftp.ack([]byte{0, opcACK, 0, 0})
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
		tftp.ack([]byte{0, opcACK, byte(i >> 8), byte(i)})
		i++
	}

	return got, nil
}
