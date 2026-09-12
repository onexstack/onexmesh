// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package bytebufferpool

import "io"

// ByteBuffer is a pooled byte buffer for append-like workloads. Obtain one with
// Get and return it with Put; its B must not be touched after Put.
type ByteBuffer struct {
	B []byte
}

// Len returns the number of bytes in the buffer.
func (b *ByteBuffer) Len() int { return len(b.B) }

// Cap returns the capacity of the buffer.
func (b *ByteBuffer) Cap() int { return cap(b.B) }

// ReadFrom implements io.ReaderFrom, appending all data read from r.
func (b *ByteBuffer) ReadFrom(r io.Reader) (int64, error) {
	p := b.B
	nStart := int64(len(p))
	nMax := int64(cap(p))
	n := nStart
	if nMax == 0 {
		nMax = 64
		p = make([]byte, nMax)
	} else {
		p = p[:nMax]
	}
	for {
		if n == nMax {
			nMax *= 2
			bNew := make([]byte, nMax)
			copy(bNew, p)
			p = bNew
		}
		nn, err := r.Read(p[n:])
		n += int64(nn)
		if err != nil {
			b.B = p[:n]
			n -= nStart
			if err == io.EOF {
				return n, nil
			}
			return n, err
		}
	}
}

// WriteTo implements io.WriterTo.
func (b *ByteBuffer) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(b.B)
	return int64(n), err
}

// Bytes returns the accumulated bytes (bytes.Buffer compatibility).
func (b *ByteBuffer) Bytes() []byte { return b.B }

// Write implements io.Writer, appending p.
func (b *ByteBuffer) Write(p []byte) (int, error) {
	b.B = append(b.B, p...)
	return len(p), nil
}

// WriteByte appends a single byte.
func (b *ByteBuffer) WriteByte(c byte) error {
	b.B = append(b.B, c)
	return nil
}

// WriteString appends s.
func (b *ByteBuffer) WriteString(s string) (int, error) {
	b.B = append(b.B, s...)
	return len(s), nil
}

// Set replaces the buffer contents with p.
func (b *ByteBuffer) Set(p []byte) {
	b.B = append(b.B[:0], p...)
}

// SetString replaces the buffer contents with s.
func (b *ByteBuffer) SetString(s string) {
	b.B = append(b.B[:0], s...)
}

// String returns the buffer contents as a string.
func (b *ByteBuffer) String() string { return string(b.B) }

// Reset empties the buffer without releasing capacity.
func (b *ByteBuffer) Reset() {
	b.B = b.B[:0]
}
