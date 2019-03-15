package mobi

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strconv"
)

// Reader allows for reading a Mobi file
type Reader struct {
	file     io.ReadSeeker
	fileStat os.FileInfo
	Mobi
}

// NewReader constructs a new reader
func NewReader(filename string) (out *Reader, err error) {
	out = &Reader{}
	file, err := os.Open(filename)
	out.file = file
	if err != nil {
		return nil, err
	}

	out.fileStat, err = file.Stat()
	if err != nil {
		return nil, err
	}

	return out, out.Parse()
}

// Parse will parse the fields of the file into this Reader
func (r *Reader) Parse() (err error) {
	if err = r.parsePdf(); err != nil {
		return
	}

	if err = r.parsePdh(); err != nil {
		return
	}

	// Check if INDX offset is set + attempt to parse INDX
	if r.Header.IndxRecodOffset > 0 {
		err = r.parseIndexRecord(r.Header.IndxRecodOffset)
		if err != nil {
			return
		}
	}

	return
}

// parseHeader reads Palm Database Format header, and record offsets
func (r *Reader) parsePdf() error {
	//First we read PDF Header, this will help us parse subsequential data
	//binary.Read will take struct and fill it with data from mobi File
	err := binary.Read(r.file, binary.BigEndian, &r.Pdf)
	if err != nil {
		return err
	}

	if r.Pdf.RecordsNum < 1 {
		return errors.New("Number of records in this file is less than 1")
	}

	r.Offsets = make([]mobiRecordOffset, r.Pdf.RecordsNum)
	err = binary.Read(r.file, binary.BigEndian, &r.Offsets)
	if err != nil {
		return err
	}

	//After the records offsets there's a 2 byte padding
	r.file.Seek(2, io.SeekCurrent)

	return nil
}

// parsePdh processes record 0 that contains PalmDoc Header, Mobi Header and Exth meta data
func (r *Reader) parsePdh() error {
	// Palm Doc Header
	// Now we go onto reading record 0 that contains Palm Doc Header, Mobi Header, Exth Header...
	binary.Read(r.file, binary.BigEndian, &r.Pdh)

	// Check and see if there's a record encryption
	if r.Pdh.Encryption != 0 {
		return errors.New("Records are encrypted")
	}

	// Mobi Header
	// Now it's time to read Mobi Header
	if r.MatchMagic(magicMobi) {
		binary.Read(r.file, binary.BigEndian, &r.Header)
	} else {
		return errors.New("Can not find MOBI header. File might be corrupt")
	}

	// Current header struct only reads 232 bytes. So if actual header lenght is greater, then we need to skip to Exth.
	Skip := int64(r.Header.HeaderLength) - int64(reflect.TypeOf(r.Header).Size())
	r.file.Seek(Skip, io.SeekCurrent)

	// Exth Record
	// To check whenever there's EXTH record or not, we need to check and see if 6th bit of r.Header.ExthFlags is set.
	if hasBit(int(r.Header.ExthFlags), 6) {
		err := r.ExthParse()

		if err != nil {
			return errors.New("Can not read EXTH record")
		}
	}

	return nil
}

func (r *Reader) parseIndexRecord(n uint32) error {
	_, err := r.OffsetToRecord(n)
	if err != nil {
		return err
	}

	RecPos, _ := r.file.Seek(0, io.SeekCurrent)

	if !r.MatchMagic(magicIndx) {
		return errors.New("Index record not found at specified at given offset")
	}
	//fmt.Printf("Index %s %v\n", r.Peek(4), RecLen)

	//if len(r.Indx) == 0 {
	r.Indx = append(r.Indx, mobiIndx{})
	//}

	idx := &r.Indx[len(r.Indx)-1]

	err = binary.Read(r.file, binary.BigEndian, idx)
	if err != nil {
		return err
	}

	/* Tagx Record Parsing + Last CNCX */
	if idx.TagxOffset != 0 {
		_, err = r.file.Seek(RecPos+int64(idx.TagxOffset), io.SeekStart)
		if err != nil {
			return err
		}

		err = r.parseTagx()
		if err != nil {
			return err
		}

		// Last CNCX record follows TAGX
		if idx.CncxRecordsCount > 0 {
			r.Cncx = mobiCncx{}
			binary.Read(r.file, binary.BigEndian, &r.Cncx.Len)

			r.Cncx.ID = make([]uint8, r.Cncx.Len)
			binary.Read(r.file, binary.LittleEndian, &r.Cncx.ID)
			r.file.Seek(1, io.SeekCurrent) //Skip 0x0 termination

			binary.Read(r.file, binary.BigEndian, &r.Cncx.NCXCount)

			// PrintStruct(r.Cncx)
		}
	}

	/* Ordt Record Parsing */
	if idx.IdxtEncoding == EncUTF16 || idx.OrdtEntriesCount > 0 {
		return errors.New("ORDT parser not implemented")
	}

	/* Ligt Record Parsing */
	if idx.LigtEntriesCount > 0 {
		return errors.New("LIGT parser not implemented")
	}

	/* Idxt Record Parsing */
	if idx.IdxtCount > 0 {
		_, err = r.file.Seek(RecPos+int64(idx.IdxtOffset), io.SeekStart)
		if err != nil {
			return err
		}

		err = r.parseIdxt(idx.IdxtCount)
		if err != nil {
			return err
		}
	}

	//CNCX Data?
	var Count = 0
	if idx.IndxType == IndxTypeNormal {
		//r.file.Seek(RecPos+int64(idx.HeaderLen), 0)

		var PTagxLen = []uint8{0}
		for i, offset := range r.Idxt.Offset {
			r.file.Seek(RecPos+int64(offset), io.SeekStart)

			// Read Byte containing the lenght of a label
			r.file.Read(PTagxLen)

			// Read label
			PTagxLabel := make([]uint8, PTagxLen[0])
			r.file.Read(PTagxLabel)

			PTagxLen1 := uint16(idx.IdxtOffset) - r.Idxt.Offset[i]
			if i+1 < len(r.Idxt.Offset) {
				PTagxLen1 = r.Idxt.Offset[i+1] - r.Idxt.Offset[i]
			}

			PTagxData := make([]uint8, PTagxLen1)
			r.file.Read(PTagxData)
			fmt.Printf("\n------ %v --------\n", i)
			r.parsePtagx(PTagxData)
			Count++
			//fmt.Printf("Len: %v | Label: %s | %v\n", PTagxLen, PTagxLabel, Count)
		}
	}

	// Check next record
	//r.OffsetToRecord(n + 1)

	//
	// Process remaining INDX records
	if idx.IndxType == IndxTypeInflection {
		r.parseIndexRecord(n + 1)
	}
	//fmt.Printf("%s", )
	// Read Tagx
	//		if idx.Tagx_Offset > 0 {
	//			err := r.parseTagx()
	//			if err != nil {
	//				return err
	//			}
	//		}

	return nil
}

// MatchMagic matches next N bytes (based on lenght of magic word)
func (r *Reader) MatchMagic(magic mobiMagicType) bool {
	if r.Peek(len(magic)).magic() == magic {
		return true
	}
	return false
}

// Peek returns next N bytes without advancing the reader.
func (r *Reader) Peek(n int) Peeker {
	buf := make([]uint8, n)
	r.file.Read(buf)
	r.file.Seek(int64(n)*-1, io.SeekCurrent)
	return buf
}

// ExthParse reads/parses Exth meta data records from file
func (r *Reader) ExthParse() error {
	// If next 4 bytes are not EXTH then we have a problem
	if !r.MatchMagic(magicExth) {
		return errors.New("Currect reading position does not contain EXTH record")
	}

	binary.Read(r.file, binary.BigEndian, &r.Exth.Identifier)
	binary.Read(r.file, binary.BigEndian, &r.Exth.HeaderLenght)
	binary.Read(r.file, binary.BigEndian, &r.Exth.RecordCount)

	r.Exth.Records = make([]mobiExthRecord, r.Exth.RecordCount)
	for i := range r.Exth.Records {
		binary.Read(r.file, binary.BigEndian, &r.Exth.Records[i].RecordType)
		binary.Read(r.file, binary.BigEndian, &r.Exth.Records[i].RecordLength)

		r.Exth.Records[i].Value = make([]uint8, r.Exth.Records[i].RecordLength-8)

		Tag := getExthMetaByTag(r.Exth.Records[i].RecordType)
		switch Tag.Type {
		case EXTH_TYPE_BINARY:
			binary.Read(r.file, binary.BigEndian, &r.Exth.Records[i].Value)
			//			fmt.Printf("%v: %v\n", Tag.Name, r.Exth.Records[i].Value)
		case EXTH_TYPE_STRING:
			binary.Read(r.file, binary.LittleEndian, &r.Exth.Records[i].Value)
			//			fmt.Printf("%v: %s\n", Tag.Name, r.Exth.Records[i].Value)
		case EXTH_TYPE_NUMERIC:
			binary.Read(r.file, binary.BigEndian, &r.Exth.Records[i].Value)
			//			fmt.Printf("%v: %d\n", Tag.Name, binary.BigEndian.Uint32(r.Exth.Records[i].Value))
		}
	}

	return nil
}

// OffsetToRecord sets reading position to record N, returns total record lenght
func (r *Reader) OffsetToRecord(nu uint32) (uint32, error) {
	n := int(nu)
	if n > int(r.Pdf.RecordsNum)-1 {
		return 0, errors.New("Record ID requested is greater than total amount of records")
	}

	RecLen := uint32(0)
	if n+1 < int(r.Pdf.RecordsNum) {
		RecLen = r.Offsets[n+1].Offset
	} else {
		RecLen = uint32(r.fileStat.Size())
	}

	_, err := r.file.Seek(int64(r.Offsets[n].Offset), io.SeekStart)

	return RecLen - r.Offsets[n].Offset, err
}

func (r *Reader) parseTagx() error {
	if !r.MatchMagic(magicTagx) {
		return errors.New("TAGX record not found at given offset")
	}

	r.Tagx = mobiTagx{}

	binary.Read(r.file, binary.BigEndian, &r.Tagx.Identifier)
	binary.Read(r.file, binary.BigEndian, &r.Tagx.HeaderLenght)
	if r.Tagx.HeaderLenght < 12 {
		return errors.New("TAGX record too short")
	}
	binary.Read(r.file, binary.BigEndian, &r.Tagx.ControlByteCount)

	TagCount := (r.Tagx.HeaderLenght - 12) / 4
	r.Tagx.Tags = make([]mobiTagxTags, TagCount)

	for i := 0; i < int(TagCount); i++ {
		err := binary.Read(r.file, binary.BigEndian, &r.Tagx.Tags[i])
		if err != nil {
			return err
		}
	}

	fmt.Println("TagX called")
	// PrintStruct(r.Tagx)

	return nil
}

func (r *Reader) parseIdxt(IdxtCount uint32) error {
	fmt.Println("parseIdxt called")
	if !r.MatchMagic(magicIdxt) {
		return errors.New("IDXT record not found at given offset")
	}

	binary.Read(r.file, binary.BigEndian, &r.Idxt.Identifier)

	r.Idxt.Offset = make([]uint16, IdxtCount)

	binary.Read(r.file, binary.BigEndian, &r.Idxt.Offset)
	//for id, _ := range r.Idxt.Offset {
	//	binary.Read(r.Buffer, binary.BigEndian, &r.Idxt.Offset[id])
	//}

	//Skip two bytes? Or skip necessary amount to reach total lenght in multiples of 4?
	r.file.Seek(2, io.SeekCurrent)

	// PrintStruct(r.Idxt)
	return nil
}

func (r *Reader) parsePtagx(data []byte) {
	//control_byte_count
	//tagx
	controlBytes := data[:r.Tagx.ControlByteCount]
	data = data[r.Tagx.ControlByteCount:]

	var Ptagx []mobiPTagx //= make([]mobiPTagx, r.Tagx.TagCount())

	for _, x := range r.Tagx.Tags {
		if x.ControlByte == 0x01 {
			controlBytes = controlBytes[1:]
			continue
		}

		value := controlBytes[0] & x.Bitmask
		if value != 0 {
			var valCount uint32
			var valBytes uint32

			if value == x.Bitmask {
				if setBits[x.Bitmask] > 1 {
					// If all bits of masked value are set and the mask has more
					// than one bit, a variable width value will follow after
					// the control bytes which defines the length of bytes (NOT
					// the value count!) which will contain the corresponding
					// variable width values.
					var consumed uint32
					valBytes, consumed = vwiDec(data, true)
					//fmt.Printf("\nConsumed %v", consumed)
					data = data[consumed:]
				} else {
					valCount = 1
				}
			} else {
				mask := x.Bitmask
				for {
					if mask&1 != 0 {
						//fmt.Printf("Break")
						break
					}
					mask >>= 1
					value >>= 1
				}
				valCount = uint32(value)
			}

			Ptagx = append(Ptagx, mobiPTagx{x.Tag, x.TagNum, valCount, valBytes})
			//						ptagx[ptagx_count].tag = tagx->tags[i].tag;
			//       ptagx[ptagx_count].tag_value_count = tagx->tags[i].values_count;
			//       ptagx[ptagx_count].value_count = value_count;
			//       ptagx[ptagx_count].value_bytes = value_bytes;

			//fmt.Printf("TAGX %v %v VC:%v VB:%v\n", x.Tag, x.TagNum, value_count, value_bytes)
		}
	}
	fmt.Printf("%+v", Ptagx)
	var IndxEntry []mobiIndxEntry
	for iz, x := range Ptagx {
		var values []uint32

		if x.ValueCount != 0 {
			// Read value_count * values_per_entry variable width values.
			fmt.Printf("\nDec: ")
			for i := 0; i < int(x.ValueCount)*int(x.TagValueCount); i++ {
				byts, consumed := vwiDec(data, true)
				data = data[consumed:]

				values = append(values, byts)
				IndxEntry = append(IndxEntry, mobiIndxEntry{x.Tag, byts})
				fmt.Printf("%v %s: %v ", iz, tagEntryMap[x.Tag], byts)
			}
		} else {
			// Convert value_bytes to variable width values.
			totalConsumed := 0
			for {
				if totalConsumed < int(x.ValueBytes) {
					byts, consumed := vwiDec(data, true)
					data = data[consumed:]

					totalConsumed += int(consumed)

					values = append(values, byts)
					IndxEntry = append(IndxEntry, mobiIndxEntry{x.Tag, byts})
				} else {
					break
				}
			}
			if totalConsumed != int(x.ValueBytes) {
				panic("Error not enough bytes are consumed. Consumed " + strconv.Itoa(totalConsumed) + " out of " + strconv.Itoa(int(x.ValueBytes)))
			}
		}
	}
	fmt.Println("---------------------------")
}
