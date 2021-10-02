package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"bffnt/bffnt_headers"
)

type BFFNT struct {
	CFNT  bffnt_headers.CFNT
	FINF  bffnt_headers.FINF
	TGLP  bffnt_headers.TGLP
	CWDHs []bffnt_headers.CWDH
	CMAPs []bffnt_headers.CMAP
	KRNG  bffnt_headers.KRNG
}

var bffntRaw []byte
var err error

func (b *BFFNT) Load(bffntFile string) {
	bffntRaw, err = ioutil.ReadFile(bffntFile)
	if err != nil {
		panic(err)
	}

	b.CFNT.Decode(bffntRaw)
	b.FINF.Decode(bffntRaw)
	b.TGLP.Decode(bffntRaw)
	b.CWDHs = bffnt_headers.DecodeCWDHs(bffntRaw, b.FINF.CWDHOffset)
	b.CMAPs = bffnt_headers.DecodeCMAPs(bffntRaw, b.FINF.CMAPOffset)
	b.KRNG.Decode(bffntRaw)
}

func (b *BFFNT) Encode() []byte {
	res := make([]byte, 0)

	cfntRaw := b.CFNT.Encode()
	tglpRaw := b.TGLP.Encode()

	cwdhStartOffset := bffnt_headers.CFNT_HEADER_SIZE + bffnt_headers.FINF_HEADER_SIZE + len(tglpRaw)
	cwdhsRaw := bffnt_headers.EncodeCWDHs(b.CWDHs, cwdhStartOffset)

	cmapStartOffset := cwdhStartOffset + len(cwdhsRaw)
	cmapsRaw := bffnt_headers.EncodeCMAPs(b.CMAPs, cmapStartOffset)
	fmt.Println("==================================")
	// _ = bffnt_headers.DecodeCMAPs(cmapsRaw, 8)

	krngRaw := b.KRNG.Encode(bffntRaw)

	// finf is encoded last because it needs to know the size of tglp and cwdhs to calculate offsets
	tglpOffset := bffnt_headers.CFNT_HEADER_SIZE + bffnt_headers.FINF_HEADER_SIZE
	cwdhOffset := tglpOffset + len(tglpRaw)
	cmapOffset := cwdhOffset + len(cwdhsRaw)
	finfRaw := b.FINF.Encode(tglpOffset+8, cwdhOffset+8, cmapOffset+8)

	totalBytes := 0
	totalBytes += len(cfntRaw)
	fmt.Println("bytes written so far:", totalBytes)

	totalBytes += len(finfRaw)
	fmt.Println("bytes written so far:", totalBytes)

	totalBytes += len(tglpRaw)
	fmt.Println("bytes written so far:", totalBytes)

	totalBytes += len(cwdhsRaw)
	fmt.Println("bytes written so far:", totalBytes)

	totalBytes += len(cmapsRaw)
	fmt.Println("bytes written so far:", totalBytes)

	totalBytes += len(krngRaw)
	fmt.Println("bytes written so far:", totalBytes)

	res = append(res, cfntRaw...)
	res = append(res, finfRaw...)
	res = append(res, tglpRaw...)
	res = append(res, cwdhsRaw...)
	res = append(res, cmapsRaw...)
	res = append(res, krngRaw...)

	return res
}

// This BFFNT file is Breath of the Wild's NormalS_00.bffnt. The goal of the
// project is to create a bffnt encoder/decoder so I can upscale this font

const (
	// testBffntFile = "/Users/kyeap/workspace/bffnt/WiiU_fonts/botw/Ancient/Ancient_00.bffnt"
	// testBffntFile = "/Users/kyeap/workspace/bffnt/WiiU_fonts/botw/Special/Special_00.bffnt"
	// testBffntFile = "/Users/kyeap/workspace/bffnt/WiiU_fonts/botw/Caption/Caption_00.bffnt"
	// testBffntFile = "/Users/kyeap/workspace/bffnt/WiiU_fonts/botw/Normal/Normal_00.bffnt"
	testBffntFile = "/home/kyeap/workspace/bffnt/WiiU_fonts/botw/NormalS/NormalS_00.bffnt"
	// testBffntFile = "/Users/kyeap/workspace/bffnt/WiiU_fonts/botw/External/External_00.bffnt"

	// testBffntFile = "/Users/kyeap/workspace/bffnt/WiiU_fonts/comicfont/Normal_00.bffnt"
	// testBffntFile = "/Users/kyeap/workspace/bffnt/WiiU_fonts/kirbysans/Normal_00.bffnt"
	// testBffntFile = "/Users/kyeap/workspace/bffnt/WiiU_fonts/kirbyscript/Normal_00.bffnt"
	// testBffntFile = "/Users/kyeap/workspace/bffnt/WiiU_fonts/popjoy_font/Normal_00.bffnt"
	// testBffntFile = "/Users/kyeap/workspace/bffnt/WiiU_fonts/turbofont/Normal_00.bffnt"
)

func main() {
	flag.BoolVar(&bffnt_headers.Debug, "d", false, "enable debug output")
	flag.Parse()

	var bffnt BFFNT
	bffnt.Load(testBffntFile)

	bffntBytes := bffnt.Encode()

	// b.CFNT.Decode(bffntRaw)
	// b.FINF.Decode(bffntRaw)
	// b.TGLP.Decode(bffntRaw)
	// b.CWDHs = bffnt_headers.DecodeCWDHs(bffntRaw, b.FINF.CWDHOffset)
	// b.CMAPs = bffnt_headers.DecodeCMAPs(bffntRaw, b.FINF.CMAPOffset)
	// b.KRNG.Decode(bffntRaw)

	err := os.WriteFile("output.bffnt", bffntBytes, 0644)
	if err != nil {
		panic(err)
	}

	return
}
