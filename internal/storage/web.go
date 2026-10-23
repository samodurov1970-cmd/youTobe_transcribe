package storage

import (
	"database/sql"
	"fmt"
)

type TrashRow struct {
	ID          int64
	VideoID     string
	ChannelName sql.NullString
	VideoURL    sql.NullString
	Reason      sql.NullString
	Action      sql.NullString
	CreatedAt   sql.NullTime
}

func ListTrash(limit int) ([]TrashRow, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := DB.Query(`
		SELECT id, video_id, channel_name, video_url, reason, action, created_at
		FROM trash
		ORDER BY id DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TrashRow
	for rows.Next() {
		var r TrashRow
		if err := rows.Scan(&r.ID, &r.VideoID, &r.ChannelName, &r.VideoURL, &r.Reason, &r.Action, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func SetTrashAction(id int64, action string) error {
	switch action {
	case "delete", "reprocess":
	default:
		return fmt.Errorf("invalid action: %s", action)
	}
	res, err := DB.Exec(`UPDATE trash SET action = $1 WHERE id = $2`, action, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}