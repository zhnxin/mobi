package mobi

import "bytes"

type mobiChapter struct {
	Id           int
	Parent       int
	Title        string
	RecordOffset int
	LabelOffset  int
	Len          int
	Html         []uint8
	SubChapters  []*mobiChapter
}

// NewChapter adds a new chapter to the output MobiBook
func (w *MobiWriter) NewChapter(title string, text []byte) *mobiChapter {
	w.chapters = append(w.chapters, mobiChapter{Id: w.chapterCount, Title: title, Html: minimizeHTML(text)})
	w.chapterCount++
	return &w.chapters[len(w.chapters)-1]
}

// AddSubChapter adds a sub-chapter to the Chapter
func (w *mobiChapter) AddSubChapter(title string, text []byte) *mobiChapter {
	w.SubChapters = append(w.SubChapters, &mobiChapter{Parent: w.Id, Title: title, Html: minimizeHTML(text)})
	return w
}

// Number of sub-chapters in this chapter
func (w *mobiChapter) SubChapterCount() int {
	return len(w.SubChapters)
}

func (w *mobiChapter) generateHTML(out *bytes.Buffer) {
	//Add check for unsupported HTML tags, characters, clean up HTML
	w.RecordOffset = out.Len()
	Len0 := out.Len()
	out.WriteString("<h1>" + w.Title + "</h1>")
	out.Write(w.Html)
	out.WriteString("<mbp:pagebreak/>")
	w.Len = out.Len() - Len0
	for i := range w.SubChapters {
		w.SubChapters[i].generateHTML(out)
	}
}
