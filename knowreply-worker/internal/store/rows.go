package store

import (
	"fmt"

	"github.com/jackc/pgx/v5"
)

// scanScored 统一扫描检索结果行。
func scanScored(rows pgx.Rows) ([]Scored, error) {
	out := make([]Scored, 0, 16)
	for rows.Next() {
		var s Scored
		if err := rows.Scan(&s.ID, &s.DocID, &s.Content, &s.HeadingPath, &s.Score); err != nil {
			return nil, fmt.Errorf("scan scored row: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate scored rows: %w", err)
	}
	return out, nil
}
