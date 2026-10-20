package analytics

import "time"

// Business rules shared by the dashboard and analytics (plan section 4).
const (
	// MinSample: aggregates over fewer employees are flagged insufficient
	// (and withheld for managers and in every group view).
	MinSample = 5
	// InactiveDays: a participant whose last attempt day is <= today-InactiveDays is inactive / stale.
	InactiveDays = 30
	// ChangeThreshold: default |delta| for significant changes and the drop threshold (latest - previous <= -15).
	ChangeThreshold = 15
	// MaxCharacteristics: max repeated `characteristics` params per request.
	MaxCharacteristics = 10
	// MaxDepartments: max repeated `departments` params on /api/analytics/departments.
	MaxDepartments = 8
	// MaxSpanDays: max inclusive window span (10 years).
	MaxSpanDays = 3660
	// QueryTimeout bounds every analytics request; on expiry the handler answers 503 analytics_timeout.
	QueryTimeout = 5 * time.Second
	// CacheTTL: per-query Redis cache lifetime (staleness bound).
	CacheTTL = 60 * time.Second
	// TZ is the business timezone: "today" and measurement days are Asia/Almaty calendar dates.
	TZ = "Asia/Almaty"
)
