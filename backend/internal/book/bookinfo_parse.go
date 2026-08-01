// Shared book-detail parsing supports fetched workflows and Explore single-book pages.
package book

import (
	"context"
	"fmt"
	"strings"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
)

func (s *Searcher) parseBookInfoResponse(ctx context.Context, src booksource.BookSource, body, baseURL string, book *Book, bookData map[string]interface{}, state analyzer.SourceState) (*Book, error) {
	an := analyzer.New(body, baseURL, s.jsVM, s.cache)
	an.SetContext(ctx)
	if bookData == nil {
		bookData = bookContext(book, src)
	}
	setAnalyzerContextWithBookData(an, src, state, bookData, book, nil, nil, baseURL)

	rules := parseRuleJSON(src.RuleBookInfo)
	if initRule := strings.TrimSpace(rules["init"]); initRule != "" {
		content, err := an.GetElement(initRule)
		if err != nil {
			return nil, fmt.Errorf("book info: init rule: %w", err)
		}
		if content == nil {
			return nil, fmt.Errorf("book info: init rule returned null")
		}
		an.SetContent(content)
	}
	if rules == nil {
		return book, nil
	}
	readField := func(rule string) string {
		setAnalyzerContextWithBookData(an, src, state, bookData, book, nil, nil, baseURL)
		return mustString(an, rule)
	}
	canReName := strings.TrimSpace(rules["canReName"]) != ""
	if name := readField(rules["name"]); name != "" && (canReName || book.Name == "") {
		book.Name = name
	}
	bookData["name"] = book.Name
	syncBookFromContext(book, bookData)
	if author := readField(rules["author"]); author != "" && (canReName || book.Author == "") {
		book.Author = author
	}
	bookData["author"] = book.Author
	syncBookFromContext(book, bookData)
	book.Kind = readField(rules["kind"])
	bookData["kind"] = book.Kind
	book.WordCount = readField(rules["wordCount"])
	bookData["wordCount"] = book.WordCount
	book.LastChapter = readField(rules["lastChapter"])
	bookData["lastChapter"] = book.LastChapter
	bookData["latestChapterTitle"] = book.LastChapter
	book.Intro = readField(rules["intro"])
	bookData["intro"] = book.Intro
	book.CoverURL = resolveURL(readField(rules["coverUrl"]), baseURL)
	bookData["coverUrl"] = book.CoverURL
	book.UpdateTime = readField(rules["updateTime"])
	bookData["updateTime"] = book.UpdateTime
	if tocURL := readField(rules["tocUrl"]); tocURL != "" {
		book.TocURL = resolveURL(tocURL, baseURL)
		bookData["tocUrl"] = book.TocURL
	}
	syncBookFromContext(book, bookData)
	return book, nil
}
