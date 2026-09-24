// Package importhistory defines the read-only listing contract shared by TXT and
// EPUB. Each store retains ownership of its SQL and lifecycle-to-status mapping.
package importhistory

// Cursor orders receipts by creation time descending, format ascending, then ID
// descending. Mutable preparation/publication timestamps never affect paging.
type Cursor struct {
	CreatedAt int64  `json:"createdAt"`
	Format    string `json:"format"`
	ID        string `json:"id"`
}

type Query struct {
	Before *Cursor
	Status string
	Limit  int
}

// Where applies the common cursor and filter to each store's history projection.
// Both projections expose id, created_at and history_status; no tables are shared.
func (q Query) Where(format string) (string, []any) {
	clause := "1=1"
	args := []any{}
	if c := q.Before; c != nil {
		switch {
		case format < c.Format:
			clause = "created_at < ?"
			args = append(args, c.CreatedAt)
		case format > c.Format:
			clause = "created_at <= ?"
			args = append(args, c.CreatedAt)
		default:
			clause = "(created_at < ? OR (created_at = ? AND id < ?))"
			args = append(args, c.CreatedAt, c.CreatedAt, c.ID)
		}
	}
	if q.Status != "" {
		clause += " AND history_status = ?"
		args = append(args, q.Status)
	}
	return clause, args
}
