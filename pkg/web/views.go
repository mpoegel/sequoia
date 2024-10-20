package web

import "time"

type FeedView struct {
	URL       string
	Timestamp time.Time
	ID        string
}
