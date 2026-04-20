package domain

import "time"

type RecurrenceRule struct {
	Frequency  RecurrenceType `json:"frequency"`
	Interval   int            `json:"interval"`
	ByWeekday  []time.Weekday `json:"by_weekday,omitempty"`
	ByMonthDay []int          `json:"by_month_day,omitempty"`
	ByMonth    []time.Month   `json:"by_month,omitempty"`
	BySetPos   []int          `json:"by_set_pos,omitempty"`
	Until      *time.Time     `json:"until,omitempty"`
}

type TaskLink struct {
	Label string
	URL   string
}

type TaskAttachmentRef struct {
	ID          string
	StorageKey  string
	Filename    string
	ContentType string
	MimeType    string
	SizeBytes   int64
	UploadedAt  time.Time
	UploadedBy  string
}
