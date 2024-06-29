package abs

import (
	"math/big"
	"reflect"
	"testing"
)

func TestANSEncoder_Encode(t *testing.T) {
	type fields struct {
		SymbolScaledFreq  map[int]*big.Int
		m                 *big.Int
		pdf               []*big.Int
		cdf               []*big.Int
		state             *big.Int
		symbolToPairIndex map[int]int
		mSubPdf           []*big.Int
	}
	type args struct {
		symbol int
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &bigIntEncoder{
				symbolScaledFreq:  tt.fields.SymbolScaledFreq,
				m:                 tt.fields.m,
				pdf:               tt.fields.pdf,
				cdf:               tt.fields.cdf,
				state:             tt.fields.state,
				symbolToPairIndex: tt.fields.symbolToPairIndex,
				mSubPdf:           tt.fields.mSubPdf,
			}
			r.Encode(tt.args.symbol)
		})
	}
}

func TestEncode_Decode(t *testing.T) {
	freqs := map[int]*big.Int{
		0: big.NewInt(3),
		1: big.NewInt(3),
		2: big.NewInt(2)}
	ansEncoder := BigIntAnsEnc(freqs)
	inputSymbols := []int{0, 1, 0, 2, 2, 0, 2, 1, 2}

	expectedSymbols := make([]int, len(inputSymbols))
	for i := 0; i < len(inputSymbols); i++ {
		expectedSymbols[len(expectedSymbols)-1-i] = inputSymbols[i]
	}

	ansEncoder.EncodeSymbols(inputSymbols)

	expectedState := big.NewInt(17910)
	if ansEncoder.state.Cmp(expectedState) != 0 {
		t.Errorf("expected %v found: %v", expectedState, ansEncoder.state)
	}

	ansDecoder := BigIntAnsDec(freqs, ansEncoder.state)

	actualSymbols := make([]int, len(expectedSymbols))
	for i := range actualSymbols {
		actualSymbols[i] = ansDecoder.Decode()
	}

	if !reflect.DeepEqual(expectedSymbols, actualSymbols) {
		t.Errorf("expected: %v found %v", expectedSymbols, actualSymbols)
	}

}
