package mobi

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"sync"
	"time"
)

const (
	uint32Max = 0xFFFFFFFF
)

// Builder allows for building of MOBI book output
type Builder interface {
	AddCover(cover, thumbnail string)
	Compression(i mobiPDHCompression)
	CSS(css string)
	NewExthRecord(recType ExthType, value interface{})
	Title(i string)
	NewChapter(title string, text []byte) Chapter
	WriteTo(out io.Writer) (n int64, err error)
}

// NewBuilder constructs a new builder
func NewBuilder() Builder {
	return &mobiBuilder{}
}

// mobiBuilder allows for writing a mobi document
type mobiBuilder struct {
	timestamp   uint32
	title       string
	compression mobiPDHCompression

	chapterCount int
	chapters     []mobiChapter

	css string

	bookHTML *bytes.Buffer

	cncxBuffer      *bytes.Buffer
	cncxLabelBuffer *bytes.Buffer

	// Text records
	records [][]byte

	embedded []EmbeddedData
	Mobi
}

// EmbType is the type of embedded data
type EmbType int

const (
	// EmbCover is a cover image
	EmbCover EmbType = iota
	// EmbThumb is a thumbnail image
	EmbThumb
)

// EmbeddedData holds an embedded blob
type EmbeddedData struct {
	Type EmbType
	Data []byte
}

func (w *mobiBuilder) embed(FileType EmbType, Data []byte) int {
	w.embedded = append(w.embedded, EmbeddedData{Type: FileType, Data: Data})
	return len(w.embedded) - 1
}

//NewExthRecord adds a new exth record to the book
func (w *mobiBuilder) NewExthRecord(recType ExthType, value interface{}) {
	w.Exth.Add(uint32(recType), value)
}

// AddCover sets the cover image
// cover and thumbnail are both filenames
func (w *mobiBuilder) AddCover(cover, thumbnail string) {
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

// Title sets the title of the book being written
func (w *mobiBuilder) Title(i string) {
	w.title = i
}

// CSS declares the stylesheet (if any) to use for the book
func (w *mobiBuilder) CSS(css string) {
	w.css = css
}

// Compression sets the compression mode to use
func (w *mobiBuilder) Compression(i mobiPDHCompression) {
	w.compression = i
}

// AddRecord adds a new record. Returns Id
func (w *mobiBuilder) AddRecord(data []uint8) Mint {
	//	fmt.Printf("Adding record : %s\n", data)
	w.records = append(w.records, data)
	return w.RecordCount() - 1
}

// RecordCount yields the number of records in the MobiWriter
func (w *mobiBuilder) RecordCount() Mint {
	return Mint(len(w.records))
}

// WriteTo will write the status of this MobiWriter to the provided Writer
func (w *mobiBuilder) WriteTo(out io.Writer) (n int64, err error) {

	bw := &binaryWriter{out: out}

	w.createTOCChapter()

	// Generate HTML file
	w.bookHTML = new(bytes.Buffer)
	stylePart := ""
	if len(w.css) > 0 {
		stylePart = fmt.Sprintf("<style>%s</style>", w.css)
	}
	w.bookHTML.WriteString(fmt.Sprintf("<html><head>%s</head><body>", stylePart))
	for i := range w.chapters {
		w.chapters[i].generateHTML(w.bookHTML)
	}
	w.bookHTML.WriteString("</body></html>")

	// Generate MOBI
	w.generateCNCX() // Generates CNCX
	w.timestamp = uint32(time.Now().Unix())

	// Generate Records
	// Record 0 - Reserve [Expand Record size in case Exth is modified by third party readers? 1024*10?]
	w.AddRecord([]uint8{0})

	// Book Records
	w.Pdh.TextLength = uint32(w.bookHTML.Len())

	w.convertHTMLToRecords()

	w.Pdh.RecordCount = w.RecordCount().UInt16() - 1

	// Index0
	w.AddRecord([]uint8{0, 0})
	w.Header.FirstNonBookIndex = w.RecordCount().UInt32()

	w.generateINDX1()
	w.generateINDX2()

	// Image
	//FirstImageIndex : array index
	//EXTH_COVER - offset from FirstImageIndex
	if w.EmbeddedCount() > 0 {
		w.Header.FirstImageIndex = w.RecordCount().UInt32()
		//		c.Mh.FirstImageIndex = i + 2
		for i, e := range w.embedded {
			w.records = append(w.records, e.Data)
			switch e.Type {
			case EmbCover:
				w.Exth.Add(EXTH_KF8COVERURI, fmt.Sprintf("kindle:embed:%04d", i+1))
				w.Exth.Add(EXTH_COVEROFFSET, i)
			case EmbThumb:
				w.Exth.Add(EXTH_THUMBOFFSET, i)
			}
		}
	} else {
		w.Header.FirstImageIndex = uint32Max
	}

	// CNCX Record

	// Resource Record
	// w.Header.FirstImageIndex = 4294967295
	// w.Header.FirstNonBookIndex = w.RecordCount().UInt32()
	w.Header.LastContentRecordNumber = w.RecordCount().UInt16() - 1
	w.Header.FlisRecordIndex = w.AddRecord(w.generateFlis()).UInt32() // Flis
	w.Header.FcisRecordIndex = w.AddRecord(w.generateFcis()).UInt32() // Fcis
	w.AddRecord([]byte{0xE9, 0x8E, 0x0D, 0x0A})                       // EOF

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
		_, err := bw.Write(w.records[i])
		if err != nil {
			return bw.written(), err
		}
	}
	return int64(bw.writtenBytes), nil
}

func (w *mobiBuilder) createTOCChapter() {
	chapters := w.chapterCount
	buf := bytes.Buffer{}
	if chapters > 0 {
		for i := range w.chapters {
			ch := w.chapters[i]
			buf.WriteString(fmt.Sprintf("<a href='#%d'>%s</a><br>", ch.ID, ch.Title))
		}
	}
	w.NewChapter("Table of Contents", buf.Bytes())
}

func (w *mobiBuilder) convertHTMLToRecords() {

	// Helper function to get the html in chunks of a useful size
	nextChunk := func() []byte {
		chunk := bytes.NewBuffer([]byte{})
		for chunk.Len() < maxRecordSize {
			// Read one character at a time to stay below the byte limit - or close to, at least
			run, _, err := w.bookHTML.ReadRune()
			if err == io.EOF {
				break
			}
			chunk.WriteRune(run)
		}
		return chunk.Bytes()
	}

	// Convert the bookHtml to nice and cozy chunks
	chunks := [][]byte{}
	for {
		chunk := nextChunk()
		if len(chunk) == 0 {
			break
		}
		chunks = append(chunks, chunk)
	}

	// Convert chunks to records in parallel, but preserving the ordering
	records := make([][]byte, len(chunks))
	wg := sync.WaitGroup{}
	wg.Add(len(chunks))
	for i := range chunks {
		go func(i int) {
			defer wg.Done()
			records[i] = makeHTMLRecord(chunks[i], w.compression)
		}(i)
	}
	wg.Wait()

	// Finally, add the records in order
	for i := range records {
		w.AddRecord(records[i])
	}
}

// makeHTMLRecord converts a slice of the html data to a record
func makeHTMLRecord(RecN []byte, compression mobiPDHCompression) []byte {
	rLen := len(RecN)
	if rLen == 0 {
		return []byte{}
	}

	if rLen > maxRecordSize { // If we overate and got too big
		Trail := rLen - maxRecordSize    // get the size difference
		RecN = append(RecN, byte(Trail)) // and put it at the end of the record, so we know how long the tail is
	} else {
		RecN = append(RecN, 0) // Otherwise, but a zero byte at the end
	}

	if compression == CompressionPalmDoc {
		RecN = palmLZ77Compress(RecN) // Optionally, compress that mofo with the chosen compression strategy
	}

	return RecN // and then return it
}

// EmbeddedCount yields the number of embedded elements
func (w *mobiBuilder) EmbeddedCount() Mint {
	return Mint(len(w.embedded))
}

func (w *mobiBuilder) generateCNCX() {
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
		mobiTagxMap[tagEntryPos],
		mobiTagxMap[tagEntryLen],
		mobiTagxMap[tagEntryNameOffset],
		mobiTagxMap[tagEntryDepthLvl],
		mobiTagxMap[tagEntryEND]}

	TagxParent := []mobiTagxTags{
		mobiTagxMap[tagEntryPos],
		mobiTagxMap[tagEntryLen],
		mobiTagxMap[tagEntryNameOffset],
		mobiTagxMap[tagEntryDepthLvl],
		mobiTagxMap[tagEntryChild1],
		mobiTagxMap[tagEntryChildN],
		mobiTagxMap[tagEntryEND]}

	TagxChild := []mobiTagxTags{
		mobiTagxMap[tagEntryPos],
		mobiTagxMap[tagEntryLen],
		mobiTagxMap[tagEntryNameOffset],
		mobiTagxMap[tagEntryDepthLvl],
		mobiTagxMap[tagEntryParent],
		mobiTagxMap[tagEntryEND]}

	var id = len(w.chapters)

	for _, node := range w.chapters {
		if node.SubChapterCount() > 0 {
			ch1 := id
			chN := id + node.SubChapterCount() - 1
			fmt.Printf("Parent: %v %v %v [CHILDREN: %v %v]\n", id, node.SubChapterCount(), node.Title, ch1, chN)
			id += node.SubChapterCount()

			cncxID := fmt.Sprintf("%03v", id)

			w.Idxt.Offset = append(w.Idxt.Offset, uint16(indxHeaderLen+w.cncxBuffer.Len()))

			w.cncxBuffer.WriteByte(byte(len(cncxID)))              // Len of ID
			w.cncxBuffer.WriteString(cncxID)                       // ID
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
			cncxID := fmt.Sprintf("%03v", w.chapterCount)
			w.Idxt.Offset = append(w.Idxt.Offset, uint16(indxHeaderLen+w.cncxBuffer.Len()))

			w.cncxBuffer.WriteByte(byte(len(cncxID)))              // Len of ID
			w.cncxBuffer.WriteString(cncxID)                       // ID
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
	id = len(w.chapters)

	for i, node := range w.chapters {
		for _, child := range node.SubChapters {
			fmt.Printf("Child: %v %v %v\n", id, i, child.Title)
			cncxID := fmt.Sprintf("%03v", w.chapterCount)
			w.Idxt.Offset = append(w.Idxt.Offset, uint16(indxHeaderLen+w.cncxBuffer.Len()))

			w.cncxBuffer.WriteByte(byte(len(cncxID)))              // Len of ID
			w.cncxBuffer.WriteString(cncxID)                       // ID
			w.cncxBuffer.WriteByte(controlByte(TagxChild)[0])      // Controll Byte
			w.cncxBuffer.Write(vwiEncInt(child.RecordOffset))      // Record offset
			w.cncxBuffer.Write(vwiEncInt(child.Len))               // Lenght of a record
			w.cncxBuffer.Write(vwiEncInt(w.cncxLabelBuffer.Len())) // Label Offset //Offset relative to CNXC record
			w.cncxLabelBuffer.Write(vwiEncInt(len(child.Title)))   // CNCXLabel lenght
			w.cncxLabelBuffer.WriteString(child.Title)             // CNCXLabel title
			w.cncxBuffer.Write(vwiEncInt(1))                       // Depth
			w.cncxBuffer.Write(vwiEncInt(i))                       // Parent
			w.chapterCount++
			id++
		}
	}
}

func (w *mobiBuilder) initPDF(bw *binaryWriter) *mobiBuilder {
	stringToBytes(underlineTitle(w.title), &w.Pdf.DatabaseName) // Set Database Name
	w.Pdf.CreationTime = w.timestamp                            // Set Time
	w.Pdf.ModificationTime = w.timestamp                        // Set Time
	stringToBytes("BOOK", &w.Pdf.Type)                          // Palm Database File Code
	stringToBytes("MOBI", &w.Pdf.Creator)                       // *
	w.Pdf.UniqueIDSeed = rand.New(rand.NewSource(9)).Uint32()   // UniqueID

	w.Pdf.RecordsNum = w.RecordCount().UInt16()

	bw.writeBinary(w.Pdf)

	Oft := uint32((w.Pdf.RecordsNum * 8) + palmDBHeaderLen + 2)

	for i := uint16(0); i < w.Pdf.RecordsNum; i++ {

		bw.writeBinary(mobiRecordOffset{Offset: Oft, UniqueID: i}) // Write
		if i == 0 {
			Oft = (uint32(w.Pdh.RecordCount) * 8) + uint32(1024*10)
		}
		if i > 0 {
			Oft += uint32(len(w.records[i]))
		}
	}

	bw.pad(2)

	return w
}

func (w *mobiBuilder) initPDH(bw *binaryWriter) *mobiBuilder {
	w.Pdh.Compression = w.compression
	w.Pdh.RecordSize = maxRecordSize

	bw.writeBinary(w.Pdh) // Write
	return w
}

func (w *mobiBuilder) initHeader(bw *binaryWriter) *mobiBuilder {
	stringToBytes("MOBI", &w.Header.Identifier)
	w.Header.HeaderLength = 232
	w.Header.MobiType = 2
	w.Header.TextEncoding = 65001
	w.Header.UniqueID = w.Pdf.UniqueIDSeed + 1
	w.Header.FileVersion = 6
	w.Header.MinVersion = 6
	w.Header.OrthographicIndex = uint32Max
	w.Header.InflectionIndex = uint32Max
	w.Header.IndexNames = uint32Max
	w.Header.Locale = 1033
	w.Header.IndexKeys = uint32Max
	w.Header.ExtraIndex0 = uint32Max
	w.Header.ExtraIndex1 = uint32Max
	w.Header.ExtraIndex2 = uint32Max
	w.Header.ExtraIndex3 = uint32Max
	w.Header.ExtraIndex4 = uint32Max
	w.Header.ExtraIndex5 = uint32Max
	w.Header.ExthFlags = 80
	w.Header.DrmOffset = uint32Max
	w.Header.DrmCount = uint32Max
	w.Header.FirstContentRecordNumber = 1
	w.Header.FcisRecordCount = 1
	w.Header.FlisRecordCount = 1

	w.Header.Unknown7 = 0
	w.Header.Unknown8 = 0

	w.Header.SrcsRecordIndex = uint32Max
	w.Header.SrcsRecordCount = 0

	w.Header.Unknown9 = uint32Max
	w.Header.Unknown10 = uint32Max
	w.Header.ExtraRecordDataFlags = 1 //1

	w.Header.FullNameLength = uint32(len(w.title))
	w.Header.FullNameOffset = uint32(palmDocHeaderLen + mobiHeaderLen + w.Exth.GetHeaderLenght() + 1)

	bw.writeBinary(w.Header) // Write
	return w
}

func (w *mobiBuilder) initExth(bw *binaryWriter) *mobiBuilder {
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
