package rans8

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"math/bits"
	"sort"
)

const probBits = 6
const probScale = 1 << probBits
const minProb = 1
const rans16L = 1 << (8 - 2)
const encodeX = (rans16L >> probBits) << 8
const mask8Bits = math.MaxUint8
const maskProbScale = probScale - 1

type RANSEncoder struct {
	Freqs   []float64
	pdf     []uint16
	cdf     []uint16
	encoded []uint8
	state   uint16
}

func (r *RANSEncoder) Encode(symbol int) {
	if len(r.Freqs) != 2 {
		panic("Freqs must be set in RANSEncoder")
	}
	if len(r.pdf) == 0 {
		r.state = rans16L // we start at the bottom of the L interval
		r.encoded = make([]uint8, 0)
		r.pdf, r.cdf = pdfCdfs(r.Freqs)
	}

	//we check if the state is already "full"
	if r.state >= encodeX*r.pdf[symbol] {
		//time to pull out bits that are done
		r.encoded = append(r.encoded, uint8(r.state&mask8Bits))
		//and we update the state
		r.state = r.state >> 8
	}

	//now we add the next symbol
	r.state = ((r.state / r.pdf[symbol]) << probBits) + (r.state % r.pdf[symbol]) + r.cdf[symbol]
}

func (r *RANSEncoder) Bytes() []byte {
	return r.encoded
}

func (r *RANSEncoder) State() uint16 {
	return r.state
}

func (r *RANSEncoder) EncodeSymbols(symbols []int) {
	for _, s := range symbols {
		r.Encode(s)
	}
}

func pdfCdfs(Freqs []float64) (pdf, cdf []uint16) {
	pdf = make([]uint16, 0)
	cdf = make([]uint16, 0)

	cdf = append(cdf, 0)
	maxFreqIndex, maxFreq := 0, 0.0
	for i, f := range Freqs {
		if f <= maxFreq {
			maxFreqIndex, maxFreq = i, f
		}
		calc := uint16(math.Round(f * float64(probScale)))

		if f > 0 && calc < minProb {
			calc = minProb
		}
		pdf = append(pdf, calc)
		cdf = append(cdf, cdf[len(cdf)-1]+calc)
	}

	// now fix rounding error
	// and since we're limiting this to a two symbol system
	// it simplifies the fixing
	freqError := probScale - int(cdf[len(cdf)-1])

	if int(pdf[maxFreqIndex])+freqError > 0 {
		pdf[maxFreqIndex] = uint16(int(pdf[maxFreqIndex]) + freqError)

		for i := maxFreqIndex + 1; i < len(cdf); i++ {
			cdf[i] = uint16(int(cdf[i]) + freqError)
		}
	} else {
		maxRed := pdf[maxFreqIndex] - 1
		left := int(maxRed) + freqError
		if maxFreqIndex == 0 {
			cdf[1] += maxRed
			cdf[2] += uint16(freqError + left)
		} else {
			cdf[1] += uint16(freqError + left)
			cdf[2] += maxRed
		}
		pdf[0] = cdf[1] - cdf[0]
		pdf[1] = cdf[2] - cdf[1]
	}

	//we do a quick check to make sure none of the pdf freq
	// are 0
	for _, f := range pdf {
		if f == 0 {
			panic(fmt.Sprintf("incompatiable freq calculated %v", 0))
		}
	}
	return
}

type RANSDecoder struct {
	Freqs        []float64
	State        uint16
	Encoded      []uint8
	encodedIndex int
	pdf          []uint16
	cdf          []uint16
	state        uint16
}

func (r *RANSDecoder) Decode() int {
	if len(r.Freqs) != 2 {
		panic("Freqs must be set in RANSDecoder")
	}
	if len(r.pdf) == 0 {
		r.encodedIndex = len(r.Encoded) - 1
		r.state = r.State // we start at the bottom of the L interval
		r.pdf, r.cdf = pdfCdfs(r.Freqs)
	}

	state := r.state & (probScale - 1)
	symbol := sort.Search(len(r.cdf), func(i int) bool { return state < r.cdf[i] }) - 1

	r.state = r.pdf[symbol]*(r.state>>probBits) + (r.state & maskProbScale) - r.cdf[symbol]

	if r.state < rans16L {
		if r.encodedIndex < 0 {
			return -1
		}
		e := r.Encoded[r.encodedIndex]
		r.encodedIndex--
		r.state = r.state<<8 | uint16(e)
	}
	return symbol
}

type Writer struct {
	buf    bytes.Buffer
	output io.Writer
}

func NewWriter(buf io.Writer) *Writer {
	return &Writer{
		buf:    bytes.Buffer{},
		output: buf,
	}
}

func (writer *Writer) Write(bs []byte) (int, error) {
	if uint64(writer.buf.Len())+uint64(len(bs)) > math.MaxInt32 {
		return 0, fmt.Errorf("max encoding size exceeded")
	}
	return writer.buf.Write(bs)
}

func (writer *Writer) Flush() error {
	// We clean out the internal buffer writing out one block.
	// The block is organized like this:
	// [one probability/encoder state/encoded bytes length/encoded bytes]
	bs := writer.buf.Bytes()
	writer.buf.Reset()

	// if there was nothing there then we write nothing and return nil
	if len(bs) == 0 {
		return nil
	}

	ones := onesCount(bs)
	if ones < 0 {
		return fmt.Errorf("more data than permitted")
	}
	p := float64(ones) / float64(len(bs)*8)
	enc := RANSEncoder{
		Freqs: []float64{1 - p, p},
	}

	for _, b := range bs {
		for i := 0; i < 8; i++ {
			enc.Encode(int(b >> i & 0x1))
		}
	}
	encoded := enc.Bytes()
	err := binary.Write(writer.output, binary.LittleEndian, p)
	if err != nil {
		return err
	}
	err = binary.Write(writer.output, binary.LittleEndian, enc.state)
	if err != nil {
		return err
	}
	err = binary.Write(writer.output, binary.LittleEndian, uint32(len(encoded)))
	if err != nil {
		return err
	}
	n, err := writer.output.Write(encoded)
	if err != nil {
		return err
	}
	if n != len(encoded) {
		return fmt.Errorf("wrote %v of %v bytes", n, len(encoded))
	}

	return nil
}

func onesCount(bytes []byte) (count int) {
	for _, b := range bytes {
		c := bits.OnesCount8(b)
		count += c
	}
	return
}

//Close flushes all remaining encodings to the io.Writer
func (writer *Writer) Close() error {
	return writer.Flush()
}

type Reader struct {
	src io.Reader
	buf []byte
}

func NewReader(buf io.Reader) *Reader {
	return &Reader{
		src: buf,
	}
}

//Read reads from the io.Reader and decodes and put the result into out
// If out is smaller than what is in the io.Reader then the number of bytes decoded and io.ErrShortBuffer is returned.
// If out is larger than what is in the io.Reader then the number of bytes decoded and io.EOF is returned.
// If out is exactly the size needed to decode a block from io.Reader then the number of bytes decoded is returned and nil for the error.
func (reader *Reader) Read(out []byte) (n int, err error) {
	if len(reader.buf) == 0 {
		// we need to ge more data
		// we read in one block
		reader.buf, err = readOneBlock(reader.src)
		if err != nil {
			return 0, err
		}
		if len(reader.buf) == 0 {
			return 0, io.ErrShortWrite
		}
	}

	//we read more if needed
	for len(reader.buf) < len(out) && err != io.EOF {
		// we need to ge more data
		// we read in one block at a time
		var more []byte
		more, err = readOneBlock(reader.src)
		if err != nil && err != io.EOF {
			return 0, err
		}
		reader.buf = append(reader.buf, more...)
	}

	n = copy(out, reader.buf)

	if n < len(reader.buf) {
		err = io.ErrShortBuffer
	}

	reader.buf = reader.buf[n:]
	return
}

func readOneBlock(src io.Reader) ([]byte, error) {
	// The block is organized like this:
	// [one probability/encoder state/encoded bytes length/encoded bytes]

	var p float64
	err := binary.Read(src, binary.LittleEndian, &p)
	if err != nil {
		return nil, err
	}

	var state uint16
	err = binary.Read(src, binary.LittleEndian, &state)
	if err != nil {
		return nil, err
	}

	var length uint32
	err = binary.Read(src, binary.LittleEndian, &length)
	if err != nil {
		return nil, err
	}

	encoded := make([]byte, length)
	n, err := src.Read(encoded)
	if err != nil {
		return nil, err
	}

	if n != int(length) {
		return nil, fmt.Errorf("failed to read in a complete block")
	}

	dec := RANSDecoder{
		Freqs:   []float64{1 - p, p},
		State:   state,
		Encoded: encoded,
	}

	symbols := make([]int, 0)
	s := dec.Decode()
	for s != -1 {
		symbols = append(symbols, s)
		s = dec.Decode()
	}

	if len(symbols)%8 != 0 {
		return nil, fmt.Errorf("decoded symbols not on byte boundary")
	}
	//the decode is actually reversed so we put into bytes
	// in reverse order

	decodeBytes := make([]byte, len(symbols)/8)
	for i := 0; i < len(symbols); i++ {
		s = symbols[len(symbols)-1-i]
		if s > 0 {
			index := i / 8
			offset := i % 8
			decodeBytes[index] |= byte(1 << offset)
		}
	}

	return decodeBytes, nil
}

func (reader *Reader) Reset(src io.Reader) error {
	reader.src = src
	reader.buf = nil
	return nil
}
