package qprintable

import (
	"io"
	"fmt"
)

func unHex(c byte) (byte, bool) {
	if byte('0') <= c && c <= byte('9') {
		return c - byte('0'), true
	}
	if byte('a') <= c && c <= byte('f') {
		return c - byte('a') + 10, true
	}
	if byte('A') <= c && c <= byte('F') {
		return c - byte('A') + 10, true
	}
	return 0, false
}

func hex(p byte) (ret [2]byte) {
	ret[0] = p / 16
	if ret[0] >= 10 {
		ret[0] = ret[0] - 10 + byte('A')
	} else {
		ret[0] = ret[0] + byte('0')
	}
	ret[1] = p % 16
	if ret[1] >= 10 {
		ret[1] = ret[1] - 10 + byte('A')
	} else {
		ret[1] = ret[1] + byte('0')
	}
	return
}

type qpDecodeStatus int

const (
	qpDecodeQuoted qpDecodeStatus = iota
	qpDecodeFirst
	qpDecodeReturn
	qpDecodeNormal
)

type qpDecoder struct {
	r      io.Reader
	status qpDecodeStatus
	temp   byte
}

func NewQuotedPrintableDecoder(r io.Reader) io.Reader {
	return &qpDecoder{
		r:      r,
		status: qpDecodeNormal,
		temp:   0,
	}
}

func (d *qpDecoder) Read(p []byte) (n int, err error) {
	readed := make([]byte, len(p), cap(p))
	readedn, err := d.r.Read(readed)
	if err != nil {
		return
	}
	n = 0
	for _, b := range readed[:readedn] {
		switch d.status {
		case qpDecodeNormal:
			if b == byte('=') {
				d.status = qpDecodeQuoted
			} else {
				p[n] = b
				n++
			}
		case qpDecodeQuoted:
			switch b {
			case byte('\n'):
				d.status = qpDecodeNormal
			case byte('\r'):
				d.status = qpDecodeReturn
			default:
				h, ok := unHex(b)
				if !ok {
					err = fmt.Errorf("can't convert %c(%d) to hex", rune(b), b)
					return
				}
				d.temp = h * 16
				d.status = qpDecodeFirst
			}
		case qpDecodeReturn:
			if b != byte('\n') {
				p[n] = d.temp
				n++
			}
			d.status = qpDecodeNormal
		case qpDecodeFirst:
			h, ok := unHex(b)
			if !ok {
				err = fmt.Errorf("can't convert %c(%d) to hex", rune(b), b)
				return
			}
			d.temp += h
			p[n] = d.temp
			n++
			d.status = qpDecodeNormal
		}
	}
	return
}

type qpEncodeStatus int

const (
	qpEncodeNormal qpEncodeStatus = iota
	qpEncodeSpace
	qpEncodeSpaceReturn
	qpEncodeReturn
)

const maxBuf = 1024

type qpEncoder struct {
	w             io.Writer
	status        qpEncodeStatus
	maxLineLength int
	lineLength    int
	nbuf          int
	buf           [maxBuf]byte
	last          byte
}

func NewQuotedPrintableEncoder(w io.Writer, maxLength int) io.WriteCloser {
	if maxLength < 3 {
		maxLength = 3
	}
	if maxLength > 76 {
		maxLength = 76
	}
	return &qpEncoder{
		w:             w,
		status:        qpEncodeNormal,
		maxLineLength: maxLength,
		lineLength:    0,
		nbuf:          0,
	}
}

func (e *qpEncoder) Write(p []byte) (n int, err error) {
	e.nbuf = 0
	var b byte
	for n, b = range p {
		switch e.status {
		case qpEncodeNormal:
			switch b {
			case byte(' '):
				fallthrough
			case byte('\t'):
				e.status = qpEncodeSpace
				e.last = b
			case byte('\r'):
				e.status = qpEncodeReturn
			case byte('\n'):
				if err = e.push(byte('\r')); err != nil {
					return
				}
				if err = e.push(byte('\n')); err != nil {
					return
				}
			case byte('='):
				if err = e.pushQuoted(byte('=')); err != nil {
					return
				}
			default:
				if err = e.pushCheck(b); err != nil {
					return
				}
			}
		case qpEncodeSpace:
			switch b {
			case byte('\r'):
				e.status = qpEncodeSpaceReturn
			case byte('\n'):
				e.status = qpEncodeNormal
				if err = e.pushQuoted(e.last); err != nil {
					return
				}
				if err = e.push('\r'); err != nil {
					return
				}
				if err = e.push('\n'); err != nil {
					return
				}
			default:
				e.status = qpEncodeNormal
				if err = e.push(e.last); err != nil {
					return
				}
				if err = e.pushCheck(b); err != nil {
					return
				}
			}
		case qpEncodeSpaceReturn:
			if b == byte('\n') {
				if err = e.pushQuoted(e.last); err != nil {
					return
				}
				if err = e.push('\r'); err != nil {
					return
				}
				if err = e.push('\n'); err != nil {
					return
				}
			} else {
				if err = e.push(e.last); err != nil {
					return
				}
				if err = e.pushQuoted(byte('\r')); err != nil {
					return
				}
				if err = e.pushCheck(b); err != nil {
					return
				}
			}
		case qpEncodeReturn:
			if b == byte('\n') {
				if err = e.push(byte('\r')); err != nil {
					return
				}
				if err = e.push(byte('\n')); err != nil {
					return
				}
			} else {
				if err = e.pushQuoted(byte('\r')); err != nil {
					return
				}
				if err = e.pushCheck(b); err != nil {
					return
				}
			}
		}
	}
	_, err = e.w.Write(e.buf[:e.nbuf])
	return
}

func (e *qpEncoder) Close() error {
	return nil
}

func (e *qpEncoder) pushCheck(p byte) error {
	if 33 <= p && p <= 126 {
		return e.push(p)
	}
	return e.pushQuoted(p)
}

func (e *qpEncoder) push(p byte) error {
	if p != byte('\r') && p != byte('\n') {
		if (e.lineLength + 1) >= e.maxLineLength {
			e.buf[e.nbuf] = byte('=')
			e.nbuf++
			e.buf[e.nbuf] = byte('\r')
			e.nbuf++
			e.buf[e.nbuf] = byte('\n')
			e.nbuf++
			e.lineLength = 0
		}
	} else {
		e.lineLength = 0
	}
	e.buf[e.nbuf] = p
	e.nbuf++
	e.lineLength++
	return e.checkAndSendBuffer()
}

func (e *qpEncoder) pushQuoted(p byte) error {
	if (e.lineLength + 3) >= e.maxLineLength {
		e.push(byte('='))
		e.push(byte('\r'))
		e.push(byte('\n'))
	}
	if err := e.push(byte('=')); err != nil {
		return err
	}
	for _, c := range hex(p) {
		if err := e.push(byte(c)); err != nil {
			return err
		}
	}
	return nil
}

func (e *qpEncoder) checkAndSendBuffer() error {
	if e.nbuf >= maxBuf {
		_, err := e.w.Write(e.buf[:])
		if err != nil {
			return err
		}
		e.nbuf = 0
	}
	return nil
}
