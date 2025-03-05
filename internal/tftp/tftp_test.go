package tftp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRRQ(t *testing.T) {
	blockLen := 512
	want := make([]byte, blockLen)
	data := []byte("abcdefghijklmnopqrstuvwxyz")
	copy(want[:len(data)], data)

	dir := t.TempDir()
	path := filepath.Join(dir, "tmp.txt")
	err := os.WriteFile(path, data, 0644)
	if err != nil {
		t.Error(err)
	}
	defer os.Remove(path)
	srvDir = dir

	head := []byte{0, OpcRRQ}
	rrq := append(head, []byte("tmp.txt")...)
	tftp, err := RRQ(rrq)
	if err != nil {
		t.Error(err)
	}

	block := make([]byte, blockLen)
	_, err = tftp.Read(block)
	if err != nil {
		t.Error(err)
	}
	for i, v := range block {
		if v != want[i] {
			println(v, want[i])
			t.Errorf("Value")
		}
	}
}
