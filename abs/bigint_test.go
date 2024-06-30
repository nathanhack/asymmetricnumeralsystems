package abs

import (
	"math/big"
	"reflect"
	"testing"
)


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
