package mobi

type tagEntry uint8

const (
	tagEntryEND                tagEntry = 0
	tagEntryPos                         = 1  // NCX | Position offset for the beginning of NCX record (filepos) Ex: Beginning of a chapter
	tagEntryLen                         = 2  // NCX | Record lenght. Ex: Chapter lenght
	tagEntryNameOffset                  = 3  // NCX | Label text offset in CNCX
	tagEntryDepthLvl                    = 4  // NCX | Depth/Level of CNCX
	tagEntryKOffs                       = 5  // NCX | kind CNCX offset
	tagEntryPosFid                      = 6  // NCX | pos:fid
	tagEntryParent                      = 21 // NCX | Parent
	tagEntryChild1                      = 22 // NCX | First child
	tagEntryChildN                      = 23 // NCX | Last child
	tagEntryImageIndex                  = 69
	tagEntryDescOffset                  = 70 // Description offset in cncx
	tagEntryAuthorOffset                = 71 // Author offset in cncx
	tagEntryImageCaptionOffset          = 72 // Image caption offset in cncx
	tagEntryImgAttrOffset               = 73 // Image attribution offset in cncx
)

var tagEntryMap = map[tagEntry]string{
	tagEntryPos:                "Offset",
	tagEntryLen:                "Lenght",
	tagEntryNameOffset:         "Label",
	tagEntryDepthLvl:           "Depth",
	tagEntryKOffs:              "Kind",
	tagEntryPosFid:             "Pos:Fid",
	tagEntryParent:             "Parent",
	tagEntryChild1:             "First Child",
	tagEntryChildN:             "Last Child",
	tagEntryImageIndex:         "Image Index",
	tagEntryDescOffset:         "Description",
	tagEntryAuthorOffset:       "Author",
	tagEntryImageCaptionOffset: "Image Caption Offset",
	tagEntryImgAttrOffset:      "Image Attr Offset"}

type mobiPTagx struct {
	Tag           tagEntry
	TagValueCount uint8
	ValueCount    uint32
	ValueBytes    uint32
}
