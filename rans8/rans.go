package rans8

import (
	"fmt"
	"math"
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
