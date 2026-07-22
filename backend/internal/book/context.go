// Shared typed JavaScript context for detail, TOC, and chapter-content rules.
package book

import (
	"encoding/json"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/sourceexec"
)

func setExecutorContext(executor *sourceexec.Executor, src booksource.BookSource, b *Book, current, next *Chapter, baseURL string) {
	setExecutorContextWithBookData(executor, src, bookContext(b, src), b, current, next, baseURL)
}

func setExecutorContextWithBookData(executor *sourceexec.Executor, src booksource.BookSource, bookData map[string]interface{}, b *Book, current, next *Chapter, baseURL string) {
	executor.SetURLContext(&analyzer.URLContext{
		Book:        bookData,
		Chapter:     chapterContextOrNil(b, current, baseURL),
		NextChapter: chapterContextOrNil(b, next, baseURL),
		JSLib:       src.JSLib,
	})
}

func setAnalyzerContext(an *analyzer.Analyzer, src booksource.BookSource, state analyzer.SourceState, b *Book, current, next *Chapter, baseURL string) {
	setAnalyzerContextWithBookData(an, src, state, bookContext(b, src), b, current, next, baseURL)
}

func setAnalyzerContextWithBookData(an *analyzer.Analyzer, src booksource.BookSource, state analyzer.SourceState, bookData map[string]interface{}, b *Book, current, next *Chapter, baseURL string) {
	setAnalyzerContextMaps(an, src, state, bookData, chapterContextOrNil(b, current, baseURL), chapterContextOrNil(b, next, baseURL))
}

func setAnalyzerContextMaps(an *analyzer.Analyzer, src booksource.BookSource, state analyzer.SourceState, bookData, chapterData, nextData map[string]interface{}) {
	an.SetJSLib(src.JSLib)
	an.SetSourceState(state)
	an.SetSourceData(sourceContext(src))
	an.SetBookDataValues(bookData)
	if chapterData != nil {
		an.SetChapterDataValues(chapterData)
	}
	if nextData != nil {
		an.SetNextChapterDataValues(nextData)
	}
}

func syncBookFromContext(b *Book, values map[string]interface{}) {
	if b == nil {
		return
	}
	stringValue := func(key string) string {
		value, _ := values[key].(string)
		return value
	}
	if value := stringValue("name"); value != "" {
		b.Name = value
	}
	if value := stringValue("author"); value != "" {
		b.Author = value
	}
	if value := stringValue("kind"); value != "" {
		b.Kind = value
	}
	if value := stringValue("intro"); value != "" {
		b.Intro = value
	}
	if value := stringValue("coverUrl"); value != "" {
		b.CoverURL = value
	}
	if value := stringValue("lastChapter"); value != "" {
		b.LastChapter = value
	}
	if value := stringValue("updateTime"); value != "" {
		b.UpdateTime = value
	}
	if value := stringValue("wordCount"); value != "" {
		b.WordCount = value
	}
}

func sourceContext(src booksource.BookSource) map[string]interface{} {
	body, _ := json.Marshal(src)
	var values map[string]interface{}
	_ = json.Unmarshal(body, &values)
	return values
}

func bookContext(b *Book, src booksource.BookSource) map[string]interface{} {
	values := map[string]interface{}{
		"bookUrl":            src.BookSourceURL,
		"tocUrl":             "",
		"origin":             src.BookSourceURL,
		"originName":         src.BookSourceName,
		"name":               "",
		"author":             "",
		"kind":               "",
		"coverUrl":           "",
		"intro":              "",
		"latestChapterTitle": "",
		"lastChapter":        "",
		"updateTime":         "",
		"wordCount":          "",
		"durChapterIndex":    0,
		"durChapterPos":      0.0,
		"totalChapterNum":    0,
	}
	if b == nil {
		return values
	}
	if b.BookURL != "" {
		values["bookUrl"] = b.BookURL
	}
	if b.TocURL != "" {
		values["tocUrl"] = b.TocURL
	}
	if b.SourceURL != "" {
		values["origin"] = b.SourceURL
	}
	if b.Origin != "" {
		values["originName"] = b.Origin
	}
	values["name"] = b.Name
	values["author"] = b.Author
	values["kind"] = b.Kind
	values["coverUrl"] = b.CoverURL
	values["intro"] = b.Intro
	values["latestChapterTitle"] = b.LastChapter
	values["lastChapter"] = b.LastChapter
	values["updateTime"] = b.UpdateTime
	values["wordCount"] = b.WordCount
	values["durChapterIndex"] = b.DurChapterIndex
	values["durChapterPos"] = b.DurChapterPos
	values["totalChapterNum"] = b.TotalChapterNum
	values["variable"] = b.VariableMap
	if b.VariableMap != "" {
		var variableMap map[string]string
		if json.Unmarshal([]byte(b.VariableMap), &variableMap) == nil {
			values["variableMap"] = variableMap
		}
	}
	values["alternateSources"] = b.AlternateSources
	values["id"] = b.ID
	return values
}

func chapterContextOrNil(b *Book, c *Chapter, baseURL string) map[string]interface{} {
	if c == nil {
		return nil
	}
	return chapterContext(b, c, baseURL)
}

func sameChapterURL(candidate, expected, baseURL string) bool {
	if candidate == "" || expected == "" {
		return false
	}
	candidateURL, _ := splitURLOptionSuffix(resolveURL(candidate, baseURL))
	expectedURL, _ := splitURLOptionSuffix(resolveURL(expected, baseURL))
	return candidateURL == expectedURL
}

func chapterContext(b *Book, c *Chapter, baseURL string) map[string]interface{} {
	bookURL := ""
	if b != nil {
		bookURL = b.BookURL
	}
	if c.BaseURL != "" {
		baseURL = c.BaseURL
	}
	return map[string]interface{}{
		"url":       c.URL,
		"title":     c.Title,
		"index":     c.Index,
		"bookUrl":   bookURL,
		"baseUrl":   baseURL,
		"isVip":     c.IsVip,
		"isVolume":  c.IsVolume,
		"isPay":     c.IsPay,
		"tag":       c.Tag,
		"wordCount": c.WordCount,
		"id":        c.ID,
	}
}
