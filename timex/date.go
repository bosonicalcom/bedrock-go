package timex

import "time"

// Date represents a date without a time component.
type Date struct {
	t time.Time
}

// NewDate creates a new Date with the given year, month, and day.
func NewDate(year, month, day int) Date {
	return Date{t: time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)}
}

// ParseDate parses a date string in the format "YYYY-MM-DD" and returns a Date.
func ParseDate(v string) (Date, error) {
	t, err := time.Parse(time.DateOnly, v)
	if err != nil {
		return Date{}, err
	}
	return Date{t: t}, nil
}

// String returns the date in the format "YYYY-MM-DD".
func (d Date) String() string {
	if d.t.IsZero() {
		return ""
	}
	return d.t.Format(time.DateOnly)
}

// Time returns the [time.Time] representation of the date.
func (d Date) Time() time.Time {
	return d.t
}
