package mobi

var mobiTagxMap = map[tagEntry]mobiTagxTags{
	tagEntryPos:        mobiTagxTags{1, 1, 1, 0},
	tagEntryLen:        mobiTagxTags{2, 1, 2, 0},
	tagEntryNameOffset: mobiTagxTags{3, 1, 4, 0},
	tagEntryDepthLvl:   mobiTagxTags{4, 1, 8, 0},
	tagEntryParent:     mobiTagxTags{21, 1, 16, 0},
	tagEntryChild1:     mobiTagxTags{22, 1, 32, 0},
	tagEntryChildN:     mobiTagxTags{23, 1, 64, 0},
	tagEntryPosFid:     mobiTagxTags{6, 2, 128, 0},
	tagEntryEND:        mobiTagxTags{0, 0, 0, 1}}

type mobiTagx struct {
	Identifier       [4]byte `format:"string"`
	HeaderLenght     uint32  `init:"Tags" op:"-12 /4"`
	ControlByteCount uint32
	Tags             []mobiTagxTags
	//[]byte //HeaderLenght - 12 | Multiple of 4

	//The tag table entries are multiple of 4 bytes. The first byte is
	//the tag, the second byte the number of values, the third byte the
	//bit mask and the fourth byte indicates the end of the control byte.
	//If the fourth byte is 0x01, all other bytes of the entry are zero.

	//Unk1 [8]uint8 //Unrealated to Tagx? || Related to CNCX Record? 8 bytes
}

type mobiTagxTags struct {
	Tag         tagEntry // /**< Tag */
	TagNum      uint8    // /**< Number of values */
	Bitmask     uint8    /**< Bitmask */
	ControlByte uint8    /**< EOF control byte */
}

func (r *mobiTagx) TagCount() int {
	return len(r.Tags)
}
