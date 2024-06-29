package abs

import (
	"fmt"
	"math/big"
	"sort"
)

//refs:
// https://arxiv.org/pdf/1311.2540.pdf
// https://fgiesen.wordp0: big.NewInt(15)ress.com/2014/02/02/rans-notes/
// https://fgiesen.wordpress.com/2014/02/18/rans-with-static-probability-distributions/
// http://cbloomrants.blogspot.com/2014/02/02-11-14-understanding-ans-10.html
// https://kedartatwawadi.github.io/post--ANS/

// populate create/calculates/returns the pdf, cdf, l, lMax, m.
// Assumptions:
//  1. the set of all symbols = len(counts)
//  2. the sum of count values is the target M
//
// Requirements:
//  1. there must not exist any key in counts whose value is zero  ( counts[x] !=0)
func populate(counts map[int]*big.Int) (pdf, cdf []*big.Int, m *big.Int, symbolToIndex, indexToSymbol map[int]int) {
	type pair struct {
		s int
		c *big.Int
	}
	//we're going to create a list of pairs(count,symbol) and sort it (decreasing)
	pdfSymbol := make([]pair, 0)
	zero := big.NewInt(0)

	for s, c := range counts {
		if c == nil {
			panic(fmt.Sprintf("counts for %v was nil", s))
		}
		if c.Cmp(zero) == 0 {
			panic(fmt.Sprintf("counts for %v was zero", s))
		}
		pdfSymbol = append(pdfSymbol, pair{s: s, c: c})
	}

	// descending order
	sort.Slice(pdfSymbol, func(i, j int) bool {
		x := pdfSymbol[i].c.Cmp(pdfSymbol[j].c)
		if x == 0 {
			return pdfSymbol[i].s < pdfSymbol[j].s
		}
		return x > 0
	})

	pdf = make([]*big.Int, len(pdfSymbol))
	for i, p := range pdfSymbol {
		pdf[i] = p.c
	}

	//now populate pdf and cdf's
	cdf = make([]*big.Int, 0, len(pdfSymbol)+1)
	cdf = append(cdf, big.NewInt(0))

	for i, p := range pdfSymbol {
		n := new(big.Int).Add(cdf[i], p.c)
		cdf = append(cdf, n)
	}

	// m is the sum of the pdf's or the last element in cdf
	m = new(big.Int).Set(cdf[len(cdf)-1])

	symbolToIndex = make(map[int]int)
	indexToSymbol = make(map[int]int)
	for i, p := range pdfSymbol {
		symbolToIndex[p.s] = i
		indexToSymbol[i] = p.s
	}

	return pdf, cdf, m, symbolToIndex, indexToSymbol
}

func BigIntAnsEnc(SymbolScaledFreq map[int]*big.Int) *bigIntEncoder {
	r := &bigIntEncoder{
		symbolScaledFreq: SymbolScaledFreq,
	}
	r.pdf, r.cdf, r.m, r.symbolToPairIndex, _ = populate(r.symbolScaledFreq)
	r.state = big.NewInt(0) // we start at the bottom of the L interval
	r.mSubPdf = make([]*big.Int, len(r.pdf))

	for i, p := range r.pdf {
		r.mSubPdf[i] = new(big.Int).Sub(r.m, p)
	}
	return r
}

type bigIntEncoder struct {
	symbolScaledFreq  map[int]*big.Int // the counts of each symbol
	m                 *big.Int         // precision (sum of SymbolScaledFreq)
	pdf               []*big.Int
	cdf               []*big.Int
	state             *big.Int
	symbolToPairIndex map[int]int
	mSubPdf           []*big.Int
}

func (r *bigIntEncoder) Encode(symbol int) {
	//we check if the state is already "full"
	index := r.symbolToPairIndex[symbol]
	pdf_s := r.pdf[index]

	//now we add the next symbol
	// state = C(symbol, state)	// expand and simplify:
	// state = r.m * state/pdf_s + cdf_s + (state mod pdf_s)
	// state = state + (r.m - pdf_s)*(state/pdf_s) + cdf_s
	// where pdf_s is the particular pdf for the s symbol
	// where cdf_s is the particular cdf for the s symbol

	// // (r.m - pdf_s) * (state/pdf_s)
	x2 := new(big.Int).Div(r.state, pdf_s)
	x2 = x2.Mul(x2, r.mSubPdf[index])

	// // then sum them all together
	cdf_s := r.cdf[r.symbolToPairIndex[symbol]]
	r.state = r.state.Add(r.state, x2)
	r.state = r.state.Add(r.state, cdf_s)

}

func (r *bigIntEncoder) State() *big.Int {
	return new(big.Int).Set(r.state)
}

func (r *bigIntEncoder) EncodeSymbols(symbols []int) {
	for _, s := range symbols {
		r.Encode(s)
	}
}

func BigIntAnsDec(SymbolScaledFreq map[int]*big.Int, State *big.Int) *bigIntDecoder {
	r := &bigIntDecoder{
		symbolScaledFreq: SymbolScaledFreq,
		state:            new(big.Int).Set(State),
	}

	r.pdf, r.cdf, r.m, _, r.indexToSymbol = populate(r.symbolScaledFreq)
	return r
}

type bigIntDecoder struct {
	symbolScaledFreq map[int]*big.Int
	pdf              []*big.Int
	cdf              []*big.Int
	state            *big.Int
	m                *big.Int
	indexToSymbol    map[int]int
}

func (r *bigIntDecoder) Decode() int {
	// remove the symbol
	// state = pdf_s*(state/m) + (state mod m) - cdf_s

	div, mod := new(big.Int).DivMod(r.state, r.m, new(big.Int))
	index := sort.Search(len(r.cdf), func(i int) bool { return mod.Cmp(r.cdf[i]) < 0 }) - 1
	symbol := r.indexToSymbol[index]

	pdf_s := r.pdf[index]
	cdf_s := r.cdf[index]

	div = div.Mul(div, pdf_s)

	r.state.Add(div, mod)
	r.state.Sub(r.state, cdf_s)

	return symbol
}
