package mobi

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"time"
)

const (
	// chSpace is the utf8 space character
	chSpace = 0x20

	// Maximum lookback offset we can pack in the 2-byte struct
	lz77WindowSize = 0x7FF

	lz77MaxChunkLen = 10
	lz77MinChunkLen = 3
)

var isNotSkipLog bool = true

func SetSkipLog(flag bool) {
	isNotSkipLog = !flag
}

func printStruct(x interface{}) {
	ref := reflect.ValueOf(x)

	if ref.Kind() == reflect.Ptr {
		ref = ref.Elem()
	}

	var CurPos uintptr
	fmt.Println("---------------------- " + ref.Type().Name() + " ----------------------")
	for i := 0; i < ref.NumField(); i++ {
		val := ref.Field(i)
		typ := ref.Type().Field(i)
		//: %-10v , int(CurPos)+int(typ.Type.Size())

		var value interface{}
		switch typ.Tag.Get("format") {
		case "bits":
			value = fmt.Sprintf("bit(%b)", val.Interface())
		case "string":
			value = fmt.Sprintf("%s", val.Interface())
		case "hex":
			value = fmt.Sprintf("% x", val.Interface())
		case "date":
			if tim, err := strconv.ParseInt(val.String(), 10, 64); err != nil {
				//BUG(fix): Check Mac/Unix timestamp format
				//If the time has the top bit set, it's an unsigned 32-bit number counting from 1st Jan 1904
				//If the time has the top bit clear, it's a signed 32-bit number counting from 1st Jan 1970.
				value = time.Unix(tim, 0)
			} else {
				value = val.Interface()
			}
		default:
			value = val.Interface()
		}

		fmt.Println(fmt.Sprintf("%-25v", typ.Name), fmt.Sprintf("%-5v:", CurPos), value)
		CurPos += typ.Type.Size()
		//}

	}
}

func hasBit(n int, pos uint) bool {
	val := n & (1 << pos)
	return (val > 0)
}

func getExthMetaByTag(tag uint32) mobiExthMeta {
	for i := 0; i < len(ExthMeta); i++ {
		if ExthMeta[i].ID == tag {
			return ExthMeta[i]
		}
	}
	return ExthMeta[0]
}

var setBits = [256]uint8{
	0, 1, 1, 2, 1, 2, 2, 3, 1, 2, 2, 3, 2, 3, 3, 4,
	1, 2, 2, 3, 2, 3, 3, 4, 2, 3, 3, 4, 3, 4, 4, 5,
	1, 2, 2, 3, 2, 3, 3, 4, 2, 3, 3, 4, 3, 4, 4, 5,
	2, 3, 3, 4, 3, 4, 4, 5, 3, 4, 4, 5, 4, 5, 5, 6,
	1, 2, 2, 3, 2, 3, 3, 4, 2, 3, 3, 4, 3, 4, 4, 5,
	2, 3, 3, 4, 3, 4, 4, 5, 3, 4, 4, 5, 4, 5, 5, 6,
	2, 3, 3, 4, 3, 4, 4, 5, 3, 4, 4, 5, 4, 5, 5, 6,
	3, 4, 4, 5, 4, 5, 5, 6, 4, 5, 5, 6, 5, 6, 6, 7,
	1, 2, 2, 3, 2, 3, 3, 4, 2, 3, 3, 4, 3, 4, 4, 5,
	2, 3, 3, 4, 3, 4, 4, 5, 3, 4, 4, 5, 4, 5, 5, 6,
	2, 3, 3, 4, 3, 4, 4, 5, 3, 4, 4, 5, 4, 5, 5, 6,
	3, 4, 4, 5, 4, 5, 5, 6, 4, 5, 5, 6, 5, 6, 6, 7,
	2, 3, 3, 4, 3, 4, 4, 5, 3, 4, 4, 5, 4, 5, 5, 6,
	3, 4, 4, 5, 4, 5, 5, 6, 4, 5, 5, 6, 5, 6, 6, 7,
	3, 4, 4, 5, 4, 5, 5, 6, 4, 5, 5, 6, 5, 6, 6, 7,
	4, 5, 5, 6, 5, 6, 6, 7, 5, 6, 6, 7, 6, 7, 7, 8,
}

// VwiDec decoders variable lenght integer. Returns value and number of bytes consumed
func vwiDec(src []uint8, forward bool) (uint32, uint32) {
	var val uint32   //val = 0
	var byts []uint8 // byts = bytearray()

	if !forward { //if not forward:
		for i, j := 0, len(src)-1; i < j; i, j = i+1, j-1 { //     src.reverse()
			src[i], src[j] = src[j], src[i]
		}
	}
	for _, bnum := range src {
		mask := ^(uint8(1) << 7)
		byts = append(byts, bnum&mask)
		if bnum>>7 == 1 {
			break
		}
	}

	if !forward { //if not forward:
		for i, j := 0, len(byts)-1; i < j; i, j = i+1, j-1 { //     src.reverse()
			byts[i], byts[j] = byts[j], byts[i]
		}
	}

	for _, Byte := range byts {
		val = val << 7
		val |= uint32(Byte)
	}

	return val, uint32(len(byts))
}

func vwiEncInt(x int) []uint8 {
	buf := make([]uint8, 64)
	z := 0
	for {
		buf[z] = byte(x) & 0x7f
		x >>= 7
		z++
		if x == 0 {
			break
		}
	}
	buf[0] |= 0x80
	for i, j := 0, z-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return buf[:z]
}

var maskToBitShifts = map[int]uint8{1: 0, 2: 1, 3: 0, 4: 2, 8: 3, 12: 2, 16: 4, 32: 5, 48: 4, 64: 6, 128: 7, 192: 6}

func controlByte(tagx []mobiTagxTags) []byte {
	var cbs []byte
	var ans uint8
	for _, tags := range tagx {
		if tags.ControlByte == 1 {
			cbs = append(cbs, ans)
			ans = 0
			continue
		}
		nvals := uint8(1)
		nentries := nvals / tags.TagNum
		shifts := maskToBitShifts[int(tags.Bitmask)]
		ans |= tags.Bitmask & (nentries << shifts)
	}
	return cbs
}

func stringToBytes(value string, output interface{}) {
	out := reflect.ValueOf(output).Elem()

	for i := 0; i < out.Type().Len(); i++ {
		if i > len(value)-1 {
			break
		}
		out.Index(i).Set(reflect.ValueOf(byte(value[i])))
	}
}

func underlineTitle(x string) string {
	x = regexp.MustCompile("[^-A-Za-z0-9]").ReplaceAllString(x, "_")
	if len(x) > 31 {
		return x[:31]
	}
	return x
}

func int32ToBytes(i uint32) []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, i)
	return buf.Bytes()
}
