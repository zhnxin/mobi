package mobi

import (
	"bytes"
	"encoding/binary"
)

func (w *mobiBuilder) chapterIsDeep() bool {
	for _, node := range w.chapters {
		if node.SubChapterCount() > 0 {
			return true
		}
	}
	return false
}

func (w *mobiBuilder) generateINDX1() {
	buf := new(bytes.Buffer)
	// Tagx
	tagx := mobiTagx{}
	if w.chapterIsDeep() {
		tagx.Tags = append(tagx.Tags, mobiTagxMap[TagEntry_Pos])
		tagx.Tags = append(tagx.Tags, mobiTagxMap[TagEntry_Len])
		tagx.Tags = append(tagx.Tags, mobiTagxMap[TagEntry_NameOffset])
		tagx.Tags = append(tagx.Tags, mobiTagxMap[TagEntry_DepthLvl])
		tagx.Tags = append(tagx.Tags, mobiTagxMap[TagEntry_Parent])
		tagx.Tags = append(tagx.Tags, mobiTagxMap[TagEntry_Child1])
		tagx.Tags = append(tagx.Tags, mobiTagxMap[TagEntry_ChildN])
		tagx.Tags = append(tagx.Tags, mobiTagxMap[TagEntry_END])
	} else {
		tagx.Tags = append(tagx.Tags, mobiTagxMap[TagEntry_Pos])
		tagx.Tags = append(tagx.Tags, mobiTagxMap[TagEntry_Len])
		tagx.Tags = append(tagx.Tags, mobiTagxMap[TagEntry_NameOffset])
		tagx.Tags = append(tagx.Tags, mobiTagxMap[TagEntry_DepthLvl])
		tagx.Tags = append(tagx.Tags, mobiTagxMap[TagEntry_END])
	}

	/*************************************/

	/*************************************/
	magicTagx.WriteTo(&tagx.Identifier)
	tagx.ControlByteCount = 1
	tagx.HeaderLenght = uint32(tagx.TagCount()*4) + 12

	TagX := new(bytes.Buffer)
	binary.Write(TagX, binary.BigEndian, tagx)

	// Indx
	//	IndxBin := new(bytes.Buffer)
	indx := mobiIndx{}
	magicIndx.WriteTo(&indx.Identifier)
	indx.HeaderLen = MOBI_INDX_HEADER_LEN
	indx.IndxType = IndxTypeInflection
	indx.IdxtCount = 1
	indx.IdxtEncoding = MOBI_ENC_UTF8
	indx.SetUnk2 = uint32Max
	indx.CncxRecordsCount = 1
	indx.IdxtEntryCount = uint32(w.chapterCount)
	indx.TagxOffset = MOBI_INDX_HEADER_LEN

	// Idxt

	/************/

	IdxtLast := len(w.Idxt.Offset)
	Offset := w.Idxt.Offset[IdxtLast-1]
	Rec := w.cncxBuffer.Bytes()[Offset-MOBI_INDX_HEADER_LEN:]

	Rec = Rec[0 : Rec[0]+1]
	RLen := len(Rec)

	Padding := (RLen + 2) % 4

	indx.IdxtOffset = MOBI_INDX_HEADER_LEN + uint32(TagX.Len()) + uint32(RLen+2+Padding) // Offset to Idxt Record
	/************/

	binary.Write(buf, binary.BigEndian, indx)
	buf.Write(TagX.Bytes())
	buf.Write(Rec)
	binary.Write(buf, binary.BigEndian, uint16(IdxtLast))

	buf.Write(make([]byte, Padding))

	buf.WriteString(magicIdxt.String())

	binary.Write(buf, binary.BigEndian, uint16(MOBI_INDX_HEADER_LEN+uint32(TagX.Len())))

	buf.Write([]byte{0, 0})
	w.Header.IndxRecodOffset = w.AddRecord(buf.Bytes()).UInt32()
}

func (w *mobiBuilder) generateINDX2() {
	buf := new(bytes.Buffer)
	indx := mobiIndx{}
	magicIndx.WriteTo(&indx.Identifier)
	indx.HeaderLen = MOBI_INDX_HEADER_LEN
	indx.IndxType = IndxTypeNormal
	indx.Unk1 = 1
	indx.IdxtEncoding = uint32Max
	indx.SetUnk2 = uint32Max
	indx.IdxtOffset = uint32(MOBI_INDX_HEADER_LEN + w.cncxBuffer.Len())
	indx.IdxtCount = uint32(len(w.Idxt.Offset))

	binary.Write(buf, binary.BigEndian, indx)
	buf.Write(w.cncxBuffer.Bytes())

	buf.WriteString(magicIdxt.String())
	for _, offset := range w.Idxt.Offset {
		//Those offsets are not relative INDX record.
		//So we need to adjust that.
		binary.Write(buf, binary.BigEndian, offset) //+MOBI_INDX_HEADER_LEN)
	}

	Padding := (len(w.Idxt.Offset) + 4) % 4
	for Padding != 0 {
		buf.Write([]byte{0})
		Padding--
	}

	w.AddRecord(buf.Bytes())
	w.AddRecord(w.cncxLabelBuffer.Bytes())
}
