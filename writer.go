package mobi

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"time"
)

type MobiWriter struct {
	file *os.File

	timestamp   uint32
	title       string
	compression mobiPDHCompression

	chapterCount int
	chapters     []mobiChapter

	css string

	bookHtml *bytes.Buffer

	cncxBuffer      *bytes.Buffer
	cncxLabelBuffer *bytes.Buffer

	// Text Records
	Records [][]uint8

	Embedded []EmbeddedData
	Mobi
}

type EmbType int

const (
	EmbCover EmbType = iota
	EmbThumb
)

type EmbeddedData struct {
	Type EmbType
	Data []byte
}

func (w *MobiWriter) embed(FileType EmbType, Data []byte) int {
	w.Embedded = append(w.Embedded, EmbeddedData{Type: FileType, Data: Data})
	return len(w.Embedded) - 1
}

//NewExthRecord adds a new exth record to the book
func (w *MobiWriter) NewExthRecord(recType ExthType, value interface{}) {
	w.Exth.Add(uint32(recType), value)
}

// AddCover sets the cover image
// cover and thumbnail are both filenames
func (w *MobiWriter) AddCover(cover, thumbnail string) {
	coverData, err := ioutil.ReadFile(cover)
	if err != nil {
		panic("Can not load file " + cover)
	}
	thumbnailData, err := ioutil.ReadFile(thumbnail)
	if err != nil {
		panic("Can not load file " + cover)
	}

	w.embed(EmbCover, coverData)
	w.embed(EmbThumb, thumbnailData)
}

// NewWriter initializes a writer. Takes a pointer to file and book title/database name
func NewWriter(filename string) (writer *MobiWriter, err error) {
	writer = &MobiWriter{}
	writer.file, err = os.Create(filename)
	if err != nil {
		return nil, err
	}
	return
}

// Title sets the title of the book being written
func (w *MobiWriter) Title(i string) *MobiWriter {
	w.title = i
	return w
}

// CSS declares the stylesheet (if any) to use for the book
func (w *MobiWriter) CSS(css string) *MobiWriter {
	w.css = css
	return w
}

// Compression sets the compression mode to use
func (w *MobiWriter) Compression(i mobiPDHCompression) *MobiWriter {
	w.compression = i
	return w
}

// AddRecord adds a new record. Returns Id
func (w *MobiWriter) AddRecord(data []uint8) Mint {
	//	fmt.Printf("Adding record : %s\n", data)
	w.Records = append(w.Records, data)
	return w.RecordCount() - 1
}

func (w *MobiWriter) RecordCount() Mint {
	return Mint(len(w.Records))
}

// Write preserves backwards compatibility of the interface and writes to a file if created with NewWriter
func (w *MobiWriter) Write() {
	if w.file == nil {
		panic("Only works if you used NewWriter to output to file. Recommend using WriteTo instead")
	}
	_, err := w.WriteTo(w.file)
	if err != nil {
		panic(err)
	}
}

// WriteTo will write the status of this MobiWriter to the provided Writer
func (w *MobiWriter) WriteTo(out io.Writer) (n int64, err error) {

	bw := &binaryWriter{out: out}

	// Generate HTML file
	w.bookHtml = new(bytes.Buffer)
	stylePart := ""
	if len(w.css) > 0 {
		stylePart = fmt.Sprintf("<style>%s</style>", w.css)
	}
	w.bookHtml.WriteString(fmt.Sprintf("<html><head>%s</head><body>", stylePart))
	for i := range w.chapters {
		w.chapters[i].generateHTML(w.bookHtml)
	}
	w.bookHtml.WriteString("</body></html>")

	// Generate MOBI
	w.generateCNCX() // Generates CNCX
	w.timestamp = uint32(time.Now().Unix())

	// Generate Records
	// Record 0 - Reserve [Expand Record size in case Exth is modified by third party readers? 1024*10?]
	w.AddRecord([]uint8{0})

	// Book Records
	w.Pdh.TextLength = uint32(w.bookHtml.Len())

	makeRecord := func(RecN []byte) []byte {
		rLen := len(RecN)
		if rLen == 0 {
			return []byte{}
		}

		if rLen > MOBI_MAX_RECORD_SIZE {
			Trail := rLen - MOBI_MAX_RECORD_SIZE
			RecN = append(RecN, byte(Trail))
		} else {
			RecN = append(RecN, 0)
		}

		if w.compression == CompressionPalmDoc {
			RecN = palmDocLZ77Pack(RecN)
		}

		return RecN
	}

	RecN := bytes.NewBuffer([]byte{})
	for {
		run, _, err := w.bookHtml.ReadRune()
		if err == io.EOF {
			w.AddRecord(makeRecord(RecN.Bytes()))
			RecN = bytes.NewBuffer([]byte{})
			break
		}
		RecN.WriteRune(run)

		if RecN.Len() >= MOBI_MAX_RECORD_SIZE {
			w.AddRecord(makeRecord(RecN.Bytes()))
			RecN = bytes.NewBuffer([]byte{})

		}
	}
	w.Pdh.RecordCount = w.RecordCount().UInt16() - 1

	// Index0
	w.AddRecord([]uint8{0, 0})
	w.Header.FirstNonBookIndex = w.RecordCount().UInt32()

	w.writeINDX_1()
	w.writeINDX_2()

	// Image
	//FirstImageIndex : array index
	//EXTH_COVER - offset from FirstImageIndex
	if w.EmbeddedCount() > 0 {
		w.Header.FirstImageIndex = w.RecordCount().UInt32()
		//		c.Mh.FirstImageIndex = i + 2
		for i, e := range w.Embedded {
			w.Records = append(w.Records, e.Data)
			switch e.Type {
			case EmbCover:
				w.Exth.Add(EXTH_KF8COVERURI, fmt.Sprintf("kindle:embed:%04d", i+1))
				w.Exth.Add(EXTH_COVEROFFSET, i)
			case EmbThumb:
				w.Exth.Add(EXTH_THUMBOFFSET, i)
			}
		}
	} else {
		w.Header.FirstImageIndex = 4294967295
	}

	// CNCX Record

	// Resource Record
	// w.Header.FirstImageIndex = 4294967295
	// w.Header.FirstNonBookIndex = w.RecordCount().UInt32()
	w.Header.LastContentRecordNumber = w.RecordCount().UInt16() - 1
	w.Header.FlisRecordIndex = w.AddRecord(w.generateFlis()).UInt32() // Flis
	w.Header.FcisRecordIndex = w.AddRecord(w.generateFcis()).UInt32() // Fcis
	w.AddRecord([]uint8{0xE9, 0x8E, 0x0D, 0x0A})                      // EOF

	w.initPDF(bw)
	w.initPDH(bw)
	w.initHeader(bw)
	w.initExth(bw)

	bw.pad(1)

	bw.Write([]byte(w.title))
	_, err = bw.seekForwardTo((int(w.Pdh.RecordCount) * 8) + 1024*10)
	if err != nil {
		return bw.written(), err
	}
	for i := 1; i < w.RecordCount().Int(); i++ {
		_, err := bw.Write(w.Records[i])
		if err != nil {
			return bw.written(), err
		}
	}
	return int64(bw.writtenBytes), nil
}

func (w *MobiWriter) EmbeddedCount() Mint {
	return Mint(len(w.Embedded))
}

func (w *MobiWriter) generateCNCX() {
	/*
		Single  [Off, Len, Label, Depth]
		Parent: [Off, Len, Label, Depth] + [FirstChild, Last Child]
		Child:  [Off, Len, Label, Depth] + [Parent]


		CNCX Structure
		0. Header 1
		1. Header 2 [Has children 3,4,5]
		2. Header 3 [Has childred 6,7]
		3. Child 1 of Header 2
		4. Child 2 of Header 2
		5. Child 3 of Header 2
		6. Child 1 of Header 3
		7. Child 2 of Header 3
	*/
	w.cncxLabelBuffer = new(bytes.Buffer)
	w.cncxBuffer = new(bytes.Buffer)
	w.chapterCount = 0

	TagxSingle := []mobiTagxTags{
		mobiTagxMap[TagEntry_Pos],
		mobiTagxMap[TagEntry_Len],
		mobiTagxMap[TagEntry_NameOffset],
		mobiTagxMap[TagEntry_DepthLvl],
		mobiTagxMap[TagEntry_END]}

	TagxParent := []mobiTagxTags{
		mobiTagxMap[TagEntry_Pos],
		mobiTagxMap[TagEntry_Len],
		mobiTagxMap[TagEntry_NameOffset],
		mobiTagxMap[TagEntry_DepthLvl],
		mobiTagxMap[TagEntry_Child1],
		mobiTagxMap[TagEntry_ChildN],
		mobiTagxMap[TagEntry_END]}

	TagxChild := []mobiTagxTags{
		mobiTagxMap[TagEntry_Pos],
		mobiTagxMap[TagEntry_Len],
		mobiTagxMap[TagEntry_NameOffset],
		mobiTagxMap[TagEntry_DepthLvl],
		mobiTagxMap[TagEntry_Parent],
		mobiTagxMap[TagEntry_END]}

	var Id = len(w.chapters)

	for _, node := range w.chapters {
		if node.SubChapterCount() > 0 {
			ch1 := Id
			chN := Id + node.SubChapterCount() - 1
			fmt.Printf("Parent: %v %v %v [CHILDREN: %v %v]\n", Id, node.SubChapterCount(), node.Title, ch1, chN)
			Id += node.SubChapterCount()

			CNCX_ID := fmt.Sprintf("%03v", Id)

			w.Idxt.Offset = append(w.Idxt.Offset, uint16(MOBI_INDX_HEADER_LEN+w.cncxBuffer.Len()))

			w.cncxBuffer.WriteByte(byte(len(CNCX_ID)))             // Len of ID
			w.cncxBuffer.WriteString(CNCX_ID)                      // ID
			w.cncxBuffer.WriteByte(controlByte(TagxParent)[0])     // Controll Byte
			w.cncxBuffer.Write(vwiEncInt(node.RecordOffset))       // Record offset
			w.cncxBuffer.Write(vwiEncInt(node.Len))                // Lenght of a record
			w.cncxBuffer.Write(vwiEncInt(w.cncxLabelBuffer.Len())) // Label Offset // Offset relative to CNXC record
			w.cncxLabelBuffer.Write(vwiEncInt(len(node.Title)))    // CNCXLabel lenght
			w.cncxLabelBuffer.WriteString(node.Title)              // CNCXLabel title
			w.cncxBuffer.Write(vwiEncInt(0))                       // Depth
			w.cncxBuffer.Write(vwiEncInt(ch1))                     // Child1
			w.cncxBuffer.Write(vwiEncInt(chN))                     // ChildN
			w.chapterCount++
		} else {
			CNCX_ID := fmt.Sprintf("%03v", w.chapterCount)
			w.Idxt.Offset = append(w.Idxt.Offset, uint16(MOBI_INDX_HEADER_LEN+w.cncxBuffer.Len()))

			w.cncxBuffer.WriteByte(byte(len(CNCX_ID)))             // Len of ID
			w.cncxBuffer.WriteString(CNCX_ID)                      // ID
			w.cncxBuffer.WriteByte(controlByte(TagxSingle)[0])     // Controll Byte
			w.cncxBuffer.Write(vwiEncInt(node.RecordOffset))       // Record offset
			w.cncxBuffer.Write(vwiEncInt(node.Len))                // Lenght of a record
			w.cncxBuffer.Write(vwiEncInt(w.cncxLabelBuffer.Len())) // Label Offset 	// Offset relative to CNXC record
			w.cncxLabelBuffer.Write(vwiEncInt(len(node.Title)))    // CNCXLabel lenght
			w.cncxLabelBuffer.WriteString(node.Title)              // CNCXLabel title
			w.cncxBuffer.Write(vwiEncInt(0))                       // Depth
			w.chapterCount++
		}

	}
	Id = len(w.chapters)

	for i, node := range w.chapters {
		for _, child := range node.SubChapters {
			fmt.Printf("Child: %v %v %v\n", Id, i, child.Title)
			CNCX_ID := fmt.Sprintf("%03v", w.chapterCount)
			w.Idxt.Offset = append(w.Idxt.Offset, uint16(MOBI_INDX_HEADER_LEN+w.cncxBuffer.Len()))

			w.cncxBuffer.WriteByte(byte(len(CNCX_ID)))             // Len of ID
			w.cncxBuffer.WriteString(CNCX_ID)                      // ID
			w.cncxBuffer.WriteByte(controlByte(TagxChild)[0])      // Controll Byte
			w.cncxBuffer.Write(vwiEncInt(child.RecordOffset))      // Record offset
			w.cncxBuffer.Write(vwiEncInt(child.Len))               // Lenght of a record
			w.cncxBuffer.Write(vwiEncInt(w.cncxLabelBuffer.Len())) // Label Offset //Offset relative to CNXC record
			w.cncxLabelBuffer.Write(vwiEncInt(len(child.Title)))   // CNCXLabel lenght
			w.cncxLabelBuffer.WriteString(child.Title)             // CNCXLabel title
			w.cncxBuffer.Write(vwiEncInt(1))                       // Depth
			w.cncxBuffer.Write(vwiEncInt(i))                       // Parent
			w.chapterCount++
			Id++
		}
	}
}

func (w *MobiWriter) initPDF(bw *binaryWriter) *MobiWriter {
	stringToBytes(underlineTitle(w.title), &w.Pdf.DatabaseName) // Set Database Name
	w.Pdf.CreationTime = w.timestamp                            // Set Time
	w.Pdf.ModificationTime = w.timestamp                        // Set Time
	stringToBytes("BOOK", &w.Pdf.Type)                          // Palm Database File Code
	stringToBytes("MOBI", &w.Pdf.Creator)                       // *
	w.Pdf.UniqueIDSeed = rand.New(rand.NewSource(9)).Uint32()   // UniqueID

	w.Pdf.RecordsNum = w.RecordCount().UInt16()

	bw.writeBinary(w.Pdf)

	Oft := uint32((w.Pdf.RecordsNum * 8) + MOBI_PALMDB_HEADER_LEN + 2)

	for i := uint16(0); i < w.Pdf.RecordsNum; i++ {

		bw.writeBinary(mobiRecordOffset{Offset: Oft, UniqueID: i}) // Write
		if i == 0 {
			Oft = (uint32(w.Pdh.RecordCount) * 8) + uint32(1024*10)
		}
		if i > 0 {
			Oft += uint32(len(w.Records[i]))
		}
	}

	bw.pad(2)

	return w
}

func (w *MobiWriter) initPDH(bw *binaryWriter) *MobiWriter {
	w.Pdh.Compression = w.compression
	w.Pdh.RecordSize = MOBI_MAX_RECORD_SIZE

	bw.writeBinary(w.Pdh) // Write
	return w
}

func (w *MobiWriter) initHeader(bw *binaryWriter) *MobiWriter {
	stringToBytes("MOBI", &w.Header.Identifier)
	w.Header.HeaderLength = 232
	w.Header.MobiType = 2
	w.Header.TextEncoding = 65001
	w.Header.UniqueID = w.Pdf.UniqueIDSeed + 1
	w.Header.FileVersion = 6
	w.Header.MinVersion = 6
	w.Header.OrthographicIndex = 4294967295
	w.Header.InflectionIndex = 4294967295
	w.Header.IndexNames = 4294967295
	w.Header.Locale = 1033
	w.Header.IndexKeys = 4294967295
	w.Header.ExtraIndex0 = 4294967295
	w.Header.ExtraIndex1 = 4294967295
	w.Header.ExtraIndex2 = 4294967295
	w.Header.ExtraIndex3 = 4294967295
	w.Header.ExtraIndex4 = 4294967295
	w.Header.ExtraIndex5 = 4294967295
	w.Header.ExthFlags = 80
	w.Header.DrmOffset = 4294967295
	w.Header.DrmCount = 4294967295
	w.Header.FirstContentRecordNumber = 1
	w.Header.FcisRecordCount = 1
	w.Header.FlisRecordCount = 1

	w.Header.Unknown7 = 0
	w.Header.Unknown8 = 0

	w.Header.SrcsRecordIndex = 4294967295
	w.Header.SrcsRecordCount = 0

	w.Header.Unknown9 = 4294967295
	w.Header.Unknown10 = 4294967295
	//w.Header.FirstCompilationDataSectionCount = 4294967295
	//w.Header.NumberOfCompilationDataSections = 4294967295
	w.Header.ExtraRecordDataFlags = 1 //1

	w.Header.FullNameLength = uint32(len(w.title))
	w.Header.FullNameOffset = uint32(MOBI_PALMDOC_HEADER_LEN + MOBI_MOBIHEADER_LEN + w.Exth.GetHeaderLenght() + 1)

	bw.writeBinary(w.Header) // Write
	return w
}

func (w *MobiWriter) initExth(bw *binaryWriter) *MobiWriter {
	stringToBytes("EXTH", &w.Exth.Identifier)
	w.Exth.HeaderLenght = 12

	for _, k := range w.Exth.Records {
		w.Exth.HeaderLenght += k.RecordLength
	}

	padding := w.Exth.HeaderLenght % 4
	w.Exth.HeaderLenght += padding

	w.Exth.RecordCount = uint32(len(w.Exth.Records))

	bw.writeBinary(w.Exth.Identifier)
	bw.writeBinary(w.Exth.HeaderLenght)
	bw.writeBinary(w.Exth.RecordCount)

	for _, k := range w.Exth.Records {
		bw.writeBinary(k.RecordType)
		bw.writeBinary(k.RecordLength)
		bw.writeBinary(k.Value)
	}

	// Add zeros to reach multiples of 4 for the header
	bw.pad(uint(padding))
	return w
}

// binaryWriter keeps track of bytes written and allows for forward 'seek' operations
type binaryWriter struct {
	out          io.Writer
	writtenBytes int
}

func (w *binaryWriter) written() int64 {
	return int64(w.writtenBytes)
}

// Write implements the Writer interface for convenience
func (w *binaryWriter) Write(data []byte) (n int, err error) {
	n, err = w.out.Write(data)
	if err == nil {
		w.writtenBytes += n
	}
	return
}

// writeBinary writes a struct as a binary structure
func (w *binaryWriter) writeBinary(data interface{}) (n int, err error) {
	buf := bytes.NewBuffer(make([]byte, 0))

	err = binary.Write(buf, binary.BigEndian, data)
	if err != nil {
		return 0, err
	}
	var written int64
	written, err = buf.WriteTo(w.out)
	n = int(written)
	w.writtenBytes += n
	return
}

// pad writes zero byte padding
func (w *binaryWriter) pad(len uint) (bytes int, err error) {
	if len == 0 {
		return 0, nil
	}
	data := make([]byte, len, len)
	return w.Write(data)
}

// seekForwardTo allows forward seeking by zero-byte padding to achieve a particular written byte count.
func (w *binaryWriter) seekForwardTo(len int) (bytes int, err error) {
	var padding uint
	if len > w.writtenBytes {
		padding = uint(len - w.writtenBytes) // Relative write forward
	}
	return w.pad(padding)
}
