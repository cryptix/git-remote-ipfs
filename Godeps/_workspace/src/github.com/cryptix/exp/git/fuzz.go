package git

// +build gofuzz

import "bytes"

func Fuzz(data []byte) int {
	_, err := DecodeObject(bytes.NewReader(data))
	if err != nil {
		return 0
	}
	return 1
}
