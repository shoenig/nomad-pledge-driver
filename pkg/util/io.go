package util

import (
	"io"
)

func NullCloser(w io.Writer) io.WriteCloser {
	return &writeCloser{w}
}

type writeCloser struct {
	io.Writer
}

func (*writeCloser) Close() error {
	return nil
}
