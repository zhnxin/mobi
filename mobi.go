package mobi

import (
	"reflect"
)

// Mobi is the core struct of a mobi document
type Mobi struct {
	Pdf     mobiPDF            // Palm Database Format: http://wiki.mobileread.com/wiki/PDB#Palm_Database_Format
	Offsets []mobiRecordOffset // Offsets for all the records. Starting from beginning of a file.
	Pdh     mobiPDH

	Header mobiHeader
	Exth   mobiExth

	//Index
	Indx  []mobiIndx
	Idxt  mobiIdxt
	Cncx  mobiCncx
	Tagx  mobiTagx
	PTagx []mobiPTagx
}

const (
	maxRecordSize    = 4096
	palmDBHeaderLen  = 78
	indxHeaderLen    = 192
	palmDocHeaderLen = 16
	mobiHeaderLen    = 232
)

type mobiRecordOffset struct {
	Offset     uint32 //The offset of record {N} from the start of the PDB of this record
	Attributes uint8  //Bit Field. The least significant four bits are used to represent the category values.
	Skip       uint8  //UniqueID is supposed to take 3 bytes, but for our inteded purposes uint16(UniqueID) should work. Let me know if there's any mobi files with more than 32767 records
	UniqueID   uint16 //The unique ID for this record. Often just a sequential count from 0
}

const (
	magicMobi     mobiMagicType = "MOBI"
	magicExth     mobiMagicType = "EXTH"
	magicHuff     mobiMagicType = "HUFF"
	magicCdic     mobiMagicType = "CDIC"
	magicFdst     mobiMagicType = "FDST"
	magicIdxt     mobiMagicType = "IDXT"
	magicIndx     mobiMagicType = "INDX"
	magicLigt     mobiMagicType = "LIGT"
	magicOrdt     mobiMagicType = "ORDT"
	magicTagx     mobiMagicType = "TAGX"
	magicFont     mobiMagicType = "FONT"
	magicAudi     mobiMagicType = "AUDI"
	magicVide     mobiMagicType = "VIDE"
	magicResc     mobiMagicType = "RESC"
	magicBoundary mobiMagicType = "BOUNDARY"
)

type mobiMagicType string

func (m mobiMagicType) String() string {
	return string(m)
}

func (m mobiMagicType) WriteTo(output interface{}) {
	out := reflect.ValueOf(output).Elem()

	if out.Type().Len() != len(m) {
		panic("Magic lenght is larger than target size")
	}

	for i := 0; i < out.Type().Len(); i++ {
		if i > len(m)-1 {
			break
		}
		out.Index(i).Set(reflect.ValueOf(byte(m[i])))
	}
}

const (
	// EncCP1252 is CP-1252 encoding
	EncCP1252 = 1252  /**< cp-1252 encoding */
	// EncUTF8 is UTF8 encoding
	EncUTF8   = 65001 /**< utf-8 encoding */
	// EncUTF16 is UTF16 encoding
	EncUTF16  = 65002 /**< utf-16 encoding */ 
)
