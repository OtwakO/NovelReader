// Content pagination errors retain the failed page and discarded partial count.
package book

import "fmt"

// ContentPaginationError reports a failed nextContentUrl workflow.
type ContentPaginationError struct {
	PageURL      string
	FailedURL    string
	PagesFetched int
	Operation    string
	Err          error
}

func (e *ContentPaginationError) Error() string {
	if e == nil {
		return "content: pagination failed"
	}
	location := paginationErrorLocation(e.PageURL, e.FailedURL)
	return fmt.Sprintf("content: %s %s after %d pages: %v", e.Operation, location, e.PagesFetched, e.Err)
}

func (e *ContentPaginationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func contentURLKey(rawURL string) string {
	urlPart, _ := splitURLOptionSuffix(rawURL)
	return urlPart
}

func newContentPaginationError(operation, pageURL, failedURL string, pages int, err error) *ContentPaginationError {
	return &ContentPaginationError{
		PageURL:      pageURL,
		FailedURL:    failedURL,
		PagesFetched: pages,
		Operation:    operation,
		Err:          err,
	}
}
