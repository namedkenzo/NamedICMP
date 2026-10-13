package outbound

import (
	"bufio"
	"errors"
)

func limitedLine(r *bufio.Reader) (string, error) {
	b, err := r.ReadSlice('\n')
	if errors.Is(err, bufio.ErrBufferFull) {
		return "", errors.New("proxy header line too long")
	}
	return string(b), err
}
