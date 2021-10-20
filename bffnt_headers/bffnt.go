package bffnt_headers

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/ioutil"
	"os"
	"sort"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Resources
// https://www.3dbrew.org/wiki/BCFNT#Version_4_.28BFFNT.29
// http://wiki.tockdom.com/wiki/BRFNT_(File_Format)
// https://github.com/KillzXGaming/Switch-Toolbox/blob/12dfbaadafb1ebcd2e07d239361039a8d05df3f7/File_Format_Library/FileFormats/Font/BXFNT/FontKerningTable.cs
// https://torinak.com/font/lsfont.html
// https://www.dafont.com/botw-hylian.font

type BFFNT struct {
	FFNT  FFNT
	FINF  FINF
	TGLP  TGLP
	CWDHs []CWDH
	CMAPs []CMAP
	KRNG  KRNG

	// Map of rune to it's index. Used to find a glyph's CWDH faster
	CWDHIndexMap map[rune]int
}

var bffntRaw []byte
var err error

func (b *BFFNT) Decode(bffntRaw []byte) {
	b.FFNT.Decode(bffntRaw)
	b.FINF.Decode(bffntRaw)
	b.TGLP.Decode(bffntRaw)
	b.CWDHs = DecodeCWDHs(bffntRaw, b.FINF.CWDHOffset)
	b.CMAPs = DecodeCMAPs(bffntRaw, b.FINF.CMAPOffset)
	b.KRNG.Decode(bffntRaw)

	b.CWDHIndexMap = make(map[rune]int, 0)
	for i, glyph := range b.GlyphIndexes() {
		b.CWDHIndexMap[rune(glyph.CharAscii)] = i
	}
}

func (b *BFFNT) Encode() []byte {
	tglpOffset := FFNT_HEADER_SIZE + FINF_HEADER_SIZE + 8
	tglpRaw := b.TGLP.Encode()

	cwdhOffset := tglpOffset + len(tglpRaw)
	cwdhsRaw := EncodeCWDHs(b.CWDHs, cwdhOffset)

	cmapOffset := cwdhOffset + len(cwdhsRaw)
	cmapsRaw := EncodeCMAPs(b.CMAPs, cmapOffset)

	finfRaw := b.FINF.Encode(tglpOffset, cwdhOffset, cmapOffset)

	krngOffset := cmapOffset + len(cmapsRaw)
	krngRaw := b.KRNG.Encode(uint32(krngOffset))

	// TODO: calculate an appriopriate blockreadnum based on sheetsize?
	fileSize := uint32(FFNT_HEADER_SIZE + len(finfRaw) + len(tglpRaw) + len(cwdhsRaw) + len(cmapsRaw) + len(krngRaw))
	ffntRaw := b.FFNT.Encode(fileSize)

	res := make([]byte, 0)
	res = append(res, ffntRaw...)
	res = append(res, finfRaw...)
	res = append(res, tglpRaw...)
	res = append(res, cwdhsRaw...)
	res = append(res, cmapsRaw...)
	res = append(res, krngRaw...)

	return res
}

// Read all valid glyphs and indexes from the CMAPs and sort them
func (b *BFFNT) GlyphIndexes() []AsciiIndexPair {
	pairSlice := make([]AsciiIndexPair, 0)
	for _, cmap := range b.CMAPs {
		for j, _ := range cmap.CharAscii {
			if cmap.CharIndex[j] != 65535 {
				p := AsciiIndexPair{
					CharAscii: cmap.CharAscii[j],
					CharIndex: cmap.CharIndex[j],
				}
				pairSlice = append(pairSlice, p)
			}
		}
	}

	sort.Slice(pairSlice, func(i, j int) bool {
		return pairSlice[i].CharIndex < pairSlice[j].CharIndex
	})

	return pairSlice
}

// This is to be used to upscale the resolution of the a texture. It will make
// the appropriate calculations based on the amount of scaling specified
// It will be up to the user to provide the upscaled images in a png format
func (b *BFFNT) Upscale(scale uint8) {
	fmt.Println("upscaling image by factor of", scale)

	b.FINF.Upscale(scale)
	b.TGLP.Upscale(scale)

	for i, _ := range b.CWDHs {
		b.CWDHs[i].Upscale(scale)
	}

	b.KRNG.Upscale(scale)
}

func Run() {
	flag.BoolVar(&Debug, "d", false, "enable debug output")
	flag.Parse()

	// scale 1 for 1280×720 (original)
	// scale 2 for 2560 × 1440
	// scale 3 for 3840 x 2160
	scale := 2
	scale = scale

	// upscaleBffnt("Ancient", "./nintendo_system_ui/botw-sheikah.ttf", scale)
	upscaleBffnt("Caption", "./nintendo_system_ui/DSi-Wii-3DS-Wii_U/FOT-RodinBokutoh-Pro-M.otf", scale)
	upscaleBffnt("Normal", "./nintendo_system_ui/DSi-Wii-3DS-Wii_U/FOT-RodinBokutoh-Pro-B.otf", scale)
	// upscaleBffnt("NormalS", "./nintendo_system_ui/DSi-Wii-3DS-Wii_U/FOT-RodinBokutoh-Pro-DB.otf", 1)
	// upscaleBffnt("External", "./nintendo_system_ui/nintendo_ext_003.ttf", scale)

	return
}

func upscaleBffnt(botwFontName string, fontFile string, scale int) {
	bffntFile := fmt.Sprintf("./WiiU_fonts/botw/%[1]s/%[1]s_00.bffnt", botwFontName)
	fmt.Println("Reading bffnt file", bffntFile)
	bffntRaw, err = ioutil.ReadFile(bffntFile)

	var bffnt BFFNT
	handleErr(err)
	bffnt.Decode(bffntRaw)

	bffnt.Upscale(uint8(scale))
	if botwFontName == "NormalS" {
		bffnt.TGLP.BaselinePosition += 6
	}

	bffnt.generateTexture(botwFontName, fontFile, scale) // This edits the CWDH

	bffnt.manuallyAdjustWidths(botwFontName, scale)

	encodedRaw := bffnt.Encode()
	fmt.Println("encoded bytes:", len(encodedRaw))

	outputBffntFile := fmt.Sprintf("%s_00_%dx_template.bffnt", botwFontName, scale)
	err = os.WriteFile(outputBffntFile, encodedRaw, 0644)
	handleErr(err)
}

func (b *BFFNT) manuallyAdjustWidths(fontName string, scale int) {
	if scale == 2 {
		switch fontName {
		case "Ancient":
		case "Caption":
			adjustBotwCaptionWidth(b)
		case "Normal":
		case "NormalS":
		case "External":
		default:
			panic("unknown font")
		}
	}
}

func adjustBotwCaptionWidth(b *BFFNT) {
	glyphWidths := b.CWDHs[0].Glyphs

	fmt.Println(glyphWidths[b.CWDHIndexMap['P']].CharWidth)
	glyphWidths[b.CWDHIndexMap['P']].CharWidth -= 4 // Play
	glyphWidths[b.CWDHIndexMap['a']].CharWidth -= 4 // Play
	glyphWidths[b.CWDHIndexMap['N']].CharWidth -= 5 // NewGame
	glyphWidths[b.CWDHIndexMap['e']].CharWidth -= 4 // NewGame
	glyphWidths[b.CWDHIndexMap['m']].CharWidth -= 2 // NewGame
	glyphWidths[b.CWDHIndexMap['o']].CharWidth -= 3 // continue
	glyphWidths[b.CWDHIndexMap['n']].CharWidth -= 2 // continue
	glyphWidths[b.CWDHIndexMap['u']].CharWidth -= 2 // continue
	glyphWidths[b.CWDHIndexMap['c']].CharWidth -= 2 // continue
	glyphWidths[b.CWDHIndexMap['h']].CharWidth -= 2 // continue
	glyphWidths[b.CWDHIndexMap['p']].CharWidth -= 2 // continue
	glyphWidths[b.CWDHIndexMap['p']].CharWidth -= 1 // Play
	glyphWidths[b.CWDHIndexMap['s']].CharWidth -= 2 // New Game
	glyphWidths[b.CWDHIndexMap['H']].CharWidth -= 4 // New Game
	glyphWidths[b.CWDHIndexMap['b']].CharWidth -= 2 // New Game
	glyphWidths[b.CWDHIndexMap['T']].CharWidth -= 2 // New Game
	glyphWidths[b.CWDHIndexMap['k']].CharWidth -= 1 // New Game
	glyphWidths[b.CWDHIndexMap['d']].CharWidth -= 1 // New Game
	glyphWidths[b.CWDHIndexMap['W']].CharWidth -= 2 // New Game
	glyphWidths[b.CWDHIndexMap['F']].CharWidth -= 1 // New Game
	glyphWidths[b.CWDHIndexMap['g']].CharWidth -= 1 // New Game
	glyphWidths[b.CWDHIndexMap['t']].CharWidth -= 1 // New Game

	glyphWidths[b.CWDHIndexMap['e']].LeftWidth -= 2 // New Game
	glyphWidths[b.CWDHIndexMap['g']].LeftWidth -= 1 // New Game
	// glyphWidths[b.CWDHIndexMap['b']].LeftWidth -= 2 // New Game

	// glyphWidths[b.CWDHIndexMap['r']].LeftWidth-- // prop

}

func (b *BFFNT) generateTexture(fontName string, fontFile string, scale int) {
	glyphIndexes := b.GlyphIndexes()

	fontSize, outlineOffset := getBotwFontSettings(fontName, scale)

	var (
		filename    = fmt.Sprintf("%s_00_%dx.png", fontName, scale)
		cellWidth   = int(b.TGLP.CellWidth)
		cellHeight  = int(b.TGLP.CellHeight)
		columnCount = int(b.TGLP.NumOfColumns)
		baseline    = int(b.TGLP.BaselinePosition) + scale
		sheetHeight = int(b.TGLP.SheetHeight)
		sheetWidth  = int(b.TGLP.SheetWidth)

		// every cell is separated by 1 px length padding at the left and top.
		realBaseline   = baseline + 1
		realCellWidth  = cellWidth + 1
		realCellHeight = cellHeight + 1
	)

	fmt.Println("Reading font file", fontFile)
	dat, err := os.ReadFile(fontFile)
	handleErr(err)

	f, err := opentype.Parse(dat)
	handleErr(err)

	face, err := opentype.NewFace(f, &opentype.FaceOptions{
		Size:    float64(fontSize),
		DPI:     144,
		Hinting: font.HintingFull,
	})
	handleErr(err)

	// drawer.MeasureString can be used to modify kerning table
	dst := image.NewAlpha(image.Rect(0, 0, sheetWidth, sheetHeight))
	glyphDrawer := font.Drawer{
		Dst:  dst,
		Src:  image.White,
		Face: face,
		Dot:  fixed.P(0, 0),
	}

	fmt.Println("face ew", face.Kern('e', 'w'))
	fmt.Println("krng ew", b.KRNG.Kern('e', 'w'))
	// fmt.Println()
	// fmt.Println("face ne", face.Kern('n', 'e'))
	// fmt.Println("krng ne", b.KRNG.Kern('n', 'e'))

	var charIndex, x, y int
	for rowIndex := 0; ; rowIndex++ {
		y = realCellHeight*rowIndex + realBaseline
		for columnIndex := 0; columnIndex < columnCount; columnIndex++ {
			x = realCellWidth * columnIndex
			glyphDrawer.Dot = fixed.P(x, y)
			// fmt.Printf("The dot is at %v\n", glyphDrawer.Dot)

			ascii := glyphIndexes[charIndex].CharAscii
			glyph := string(rune(asciiToGlyph(fontName, ascii)))
			_, glyphHasEntryInFontFile := face.GlyphAdvance(rune(asciiToGlyph(fontName, ascii)))
			if !glyphHasEntryInFontFile {
				fmt.Println(string(glyph), "has no entry")
				panic("no entry")
			}

			glyphBoundAtDot, _ := glyphDrawer.BoundString(glyph)
			// fmt.Println(x, glyphBoundAtDot.Min.X, glyphBoundAtDot.Min.Y, glyphBoundAtDot.Max.X, glyphBoundAtDot.Max.Y)

			// TODO: make this work with multiple CWDHs
			// calculate glyph x offset in it's cell so that there is only 1
			// pixel length between the cell and the left most pixel of the
			// glyph we are abount to draw. Generally the characters are draw
			// to the right of the Dot but its possible for this to be
			// negative. e.x. character j's left most pixel falls to the left
			// of the dot.
			leftAlignOffset := int(glyphBoundAtDot.Min.X/64) - x

			// Drawing new glyphs means we should update the CWDH. If a glyph's
			// recorded width is smaller than the one drawn it will get cut off
			// when rendering in the game.
			newGlyphWidth := int(glyphBoundAtDot.Max.X/64) - int(glyphBoundAtDot.Min.X/64) + 1
			newGlyphWidth += 2 * outlineOffset // usually 0 except for botw NormalS, because the font has an outline
			if newGlyphWidth > 255 {           // MaxUint8
				panic("BFFNT's maximum glyph width is 255 (MaxUint8)")
			}

			// Measure how far the dot would travel if a character is printed
			// we can use this to dial in the character width.
			newCharWidth := int(glyphDrawer.MeasureString(glyph) / 64)
			if newCharWidth > 255 { // MaxUint8
				panic("BFFNT's maximum char width is 255 (MaxUint8)")
			}

			glyphCWDH := b.CWDHs[0].Glyphs[charIndex]
			// It looks like that nintendo might have custom spacing, if the
			// difference is too big do not update CWDH
			// if math.Abs(float64(leftAlignOffset-int(glyphCWDH.LeftWidth))) <= float64(scale+1) {
			// 	fmt.Println("left ", glyph, leftAlignOffset, glyphCWDH.LeftWidth)
			// 	glyphCWDH.LeftWidth = int8(leftAlignOffset)
			// }
			// if math.Abs(float64(newCharWidth-int(glyphCWDH.CharWidth))) <= float64(scale+1) {
			// 	fmt.Println("char ", glyph, newCharWidth, glyphCWDH.CharWidth)
			// 	glyphCWDH.CharWidth = uint8(newCharWidth)
			// }
			// fmt.Println("glyph", glyph, newGlyphWidth, glyphCWDH.GlyphWidth)
			glyphCWDH.GlyphWidth = uint8(newGlyphWidth)

			y_nintendo := y - scale // manual adjust to compensate y difference between nintendo font generator and mine.
			glyphDrawer.Dot = fixed.P(x-leftAlignOffset+(outlineOffset)+1, y_nintendo)
			glyphDrawer.DrawString(glyph)

			charIndex++

			// Exit when no more characters
			if charIndex == 95 {
				// if charIndex == len(glyphIndexes) {
				goto writePng
			}
		}
	}

writePng:
	if Debug {
		// draw grid lines. Good for debugging.
		for x := 0; x < int(b.TGLP.SheetWidth); x += realCellWidth {
			drawVerticalLine(dst, x, 0, int(b.TGLP.SheetHeight)) // draw columns
		}
		for y := 0; y < int(b.TGLP.SheetHeight); y += realCellHeight {
			drawHorizontalLine(dst, 0, y, int(b.TGLP.SheetWidth)) // draw rows
		}
		for y := int(b.TGLP.BaselinePosition) + 1; y < int(b.TGLP.SheetHeight); y += realCellHeight {
			drawHorizontalLine(dst, 0, y, int(b.TGLP.SheetWidth)) // draw baseline
		}
	}

	_ = os.Remove(filename)

	fmt.Println("wrote glyphs to", filename)
	textureFile, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, 0644)
	handleErr(err)
	err = png.Encode(textureFile, dst)
	handleErr(err)
}

// Manual adjustments for each font to closely resemble the original
func getBotwFontSettings(fontName string, scale int) (fontSize int, outlineOffset int) {
	switch fontName {
	case "Ancient":
		fontSize = 6 * scale

	case "Caption":
		fontSize = 8 * scale

	case "Normal":
		fontSize = 15 * scale // 2k

	case "NormalS":
		// This is what should be the proper setting for botw NormalS. However,
		// there is a bug that stretches the words on the mini map if the
		// textures are not the same width as the original.
		// fontSize = 9 * scale
		// outlineOffset = 3 * scale // NormalS Characters have a 3px wide outline with 25% opacaity. I use GIMP.

		// Boost the font size and minimize the opacity outline to let
		// the character fill out the bounds of the texture as much as
		// possible.
		fontSize = 11 * scale
		outlineOffset = 1
		// the baseline will be manually adjusted in tglp

	case "External":
		fontSize = 15 * scale

	default:
		panic("file texture generation settings unknown")
	}

	return
}

// In most cases the ascii code maps to the correct glyph in the font file. For
// some glyphs, the ascii does not match the glyph in the font file (because we
// don't have the exact font file nintendo used). If the font file stil has the
// correct glyph at a different index we can create a manual mapping here.  No
// manual mapping means the ascii maps to the correct index in the font file.
func asciiToGlyph(fontName string, ascii uint16) uint16 {
	var asciiToGlyphMap map[uint16]uint16
	switch fontName {
	case "Ancient":
	case "Caption":
	case "Normal":
	case "NormalS":
	case "External":
		asciiToGlyphMap = getBotwExternalMap()
	default:
		panic("unknown font mapping")
	}

	glyphIndex, manualMappingExists := asciiToGlyphMap[ascii]
	if manualMappingExists {
		return glyphIndex
	}

	return ascii
}

// mapping botw external font character indexes to nintendo_ext_003.ttf
func getBotwExternalMap() map[uint16]uint16 {
	botwExternalMapping := make(map[uint16]uint16, 0)

	botwExternalMapping[57408] = 57568 // A
	botwExternalMapping[57409] = 57569 // B
	botwExternalMapping[57410] = 57570 // X
	botwExternalMapping[57411] = 57571 // Y
	botwExternalMapping[57412] = 57572 // L
	botwExternalMapping[57413] = 57573 // R
	botwExternalMapping[57414] = 57574 // ZL
	botwExternalMapping[57415] = 57575 // ZR
	botwExternalMapping[57416] = 57587 // Power
	botwExternalMapping[57417] = 57616 // D-pad
	botwExternalMapping[57418] = 57588 // Home
	botwExternalMapping[57419] = 57583 // +
	botwExternalMapping[57420] = 57584 // -

	botwExternalMapping[57424] = 57473 // Ljoy down
	botwExternalMapping[57425] = 57474 // Rjoy down
	botwExternalMapping[57426] = 57473 // Ljoy up
	botwExternalMapping[57427] = 57474 // Rjoy up
	botwExternalMapping[57428] = 57473 // Ljoy left-right
	botwExternalMapping[57429] = 57474 // Rjoy left-right
	botwExternalMapping[57430] = 57473 // Ljoy press-down
	botwExternalMapping[57431] = 57474 // Rjoy press-down
	botwExternalMapping[57432] = 57473 // Ljoy right
	botwExternalMapping[57433] = 57474 // Rjoy right
	botwExternalMapping[57434] = 57473 // Ljoy left
	botwExternalMapping[57435] = 57473 // Rjoy left
	botwExternalMapping[57437] = 57473 // Rjoy up-down
	botwExternalMapping[57438] = 57473 // Ljoy
	botwExternalMapping[57439] = 57473 // Rjoy
	botwExternalMapping[57440] = 0     // D-pad up
	botwExternalMapping[57441] = 0     // D-pad down
	botwExternalMapping[57442] = 0     // D-pad left
	botwExternalMapping[57443] = 0     // D-pad right
	botwExternalMapping[57444] = 0     // D-pad up-down
	botwExternalMapping[57445] = 0     // D-pad left-right
	// (34, 57446)
	// (35, 57447)
	// (36, 57475)
	// (37, 57476)
	// (38, 57477)
	// (39, 57478)
	// (40, 57479)
	// (41, 57480)
	// (42, 57481)
	// (43, 57482)
	// (44, 57483)
	// (45, 57484)
	// (46, 57485)
	// (47, 57486)
	// (48, 57487)

	return botwExternalMapping
}

func drawHorizontalLine(img *image.Alpha, x1, y, x2 int) {
	for ; x1 <= x2; x1++ {
		img.Set(x1, y, color.Opaque)
	}
}

func drawVerticalLine(img *image.Alpha, x, y1, y2 int) {
	for ; y1 <= y2; y1++ {
		img.Set(x, y1, color.Opaque)
	}
}
