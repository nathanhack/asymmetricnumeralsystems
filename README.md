# asymmetricnumblsystems

*asymmetricnumblsystems* - Is a set of Asymmetric Numeral Systems entropy encoding tools

Currently, implemented:

* rANS - Ranged Asymmetric Numeral Systems

### rANS

This work is based on the work from:

https://github.com/FGlazov/Python-rANSCoder
https://github.com/rygorous/ryg_rans

There are two RANS: 8 bit and 32 bit. The bits refer to the expected input/output array type: in the case of the 8 bit
the input/output is an array of bytes, and in the case of 32 bits the input/output is an array of uint32's.

### Getting Started

```
import "github.com/nathanhack/asymmetricnumberalsystems/rans8
```

Then after determining the frequency of zeros and ones create an encoder

```
ansEncoder := RANSEncoder{
    Freqs: []float64{zerosFreq,onesFreq},
}
```
Then And encode the symbols:

```
for _, s := range intSymbols {
    ansEncoder.Encode(s)
}
```

