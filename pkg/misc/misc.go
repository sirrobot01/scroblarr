package misc

import "time"

func CalculateProgress(viewOffset, duration int64) float64 {
	if duration == 0 {
		return 0
	}
	return float64(viewOffset) / float64(duration) * 100.0
}

func ParseISO8601(iso8601 string) int64 {
	// Assuming the ISO8601 string is in the format "YYYY-MM-DDTHH:MM:SSZ"
	// This function should be adjusted based on the actual format used
	t, err := time.Parse(time.RFC3339, iso8601)
	if err != nil {
		return 0 // Handle error appropriately in production code
	}
	return t.Unix() * 1000 // Convert to milliseconds
}
