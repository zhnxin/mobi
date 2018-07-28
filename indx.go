package mobi

const (
	// IndxTypeNormal indicates normal index type
	IndxTypeNormal uint32 = 0
	// IndxTypeInflection indicates inflection index type
	IndxTypeInflection uint32 = 2
)

type mobiIndx struct {
	Identifier       [4]byte `format:"string"`
	HeaderLen        uint32
	Unk0             uint32
	Unk1             uint32 /* 1 when inflection is normal? */
	IndxType         uint32 /* 12: 0 - normal, 2 - inflection */
	IdxtOffset       uint32 /* 20: IDXT offset */
	IdxtCount        uint32 /* 24: entries count */
	IdxtEncoding     uint32 /* 28: encoding */
	SetUnk2          uint32 //-1
	IdxtEntryCount   uint32 /* 36: total entries count */
	OrdtOffset       uint32
	LigtOffset       uint32
	LigtEntriesCount uint32 /* 48: LIGT entries count */
	CncxRecordsCount uint32 /* 52: CNCX entries count */
	Unk3             [108]byte
	OrdtType         uint32 /* 164: ORDT type */
	OrdtEntriesCount uint32 /* 168: ORDT entries count */
	Ordt1Offset      uint32 /* 172: ORDT1 offset */
	Ordt2Offset      uint32 /* 176: ORDT2 offset */
	TagxOffset       uint32 /* 180: */
	Unk4             uint32 /* 184: */ /* ? Default index string offset ? */
	Unk5             uint32 /* 188: */ /* ? Default index string length ? */
}

type mobiIndxEntry struct {
	EntryID    tagEntry
	EntryValue uint32
}
