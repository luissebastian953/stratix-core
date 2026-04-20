package domain

type RecurrenceType string

const (
	RecurrenceTypeNone    RecurrenceType = "none"
	RecurrenceTypeDaily   RecurrenceType = "daily"
	RecurrenceTypeWeekly  RecurrenceType = "weekly"
	RecurrenceTypeMonthly RecurrenceType = "monthly"
)
