package mobi

import (
	"bytes"
	"fmt"
)

// Chapter lets you add sub-chapters to the book
type Chapter interface {
	AddSubChapter(title string, text []byte) Chapter
}

type mobiChapter struct {
	ID           int
	Parent       int
	Title        string
	RecordOffset int
	LabelOffset  int
	Len          int
	HTML         []byte
	SubChapters  []*mobiChapter

	subChapter bool
}

// NewChapter adds a new chapter to the output MobiBook
func (w *mobiBuilder) NewChapter(title string, text []byte) Chapter {
	w.chapters = append(w.chapters, mobiChapter{ID: w.chapterCount, Title: title, HTML: text})
	w.chapterCount++
	return &w.chapters[len(w.chapters)-1]
}

// AddSubChapter adds a sub-chapter to the Chapter and returns the parent chapter back again
func (w *mobiChapter) AddSubChapter(title string, text []byte) Chapter {
	w.SubChapters = append(w.SubChapters, &mobiChapter{Parent: w.ID, Title: title, HTML: text, subChapter: true})
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
	if !w.subChapter {
		// main chapter, write TOC target
		out.WriteString(fmt.Sprintf("<a name='%d' id='%d'></a>", w.ID, w.ID))
	}
	out.WriteString("<h1>" + w.Title + "</h1>")
	out.Write(w.HTML)
	out.WriteString("<mbp:pagebreak/>")
	w.Len = out.Len() - Len0
	for i := range w.SubChapters {
		w.SubChapters[i].generateHTML(out)
	}
}
