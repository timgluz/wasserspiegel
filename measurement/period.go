package measurement

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/sosodev/duration"
)

var ErrInvalidEpoch = fmt.Errorf("invalid epoch value")

func CurrentEpoch() Epoch {
	return Epoch(time.Now().Unix())
}

type Period struct {
	Start Epoch `json:"start"`
	End   Epoch `json:"end"`
}

func (p *Period) IsValid() bool {
	return p.Start < p.End
}

func (p *Period) String() string {
	dtDuration := time.Unix(int64(p.End), 0).Sub(time.Unix(int64(p.Start), 0))

	isoDuration := duration.FromTimeDuration(dtDuration)
	return isoDuration.String()
}

func NewFromISO8601Duration(periodStr string) (*Period, error) {
	end := CurrentEpoch() // Use current epoch as the end time
	start, err := ParseISO8601Duration(periodStr, end)
	if err != nil {
		return nil, err
	}

	return &Period{
		Start: start,
		End:   end,
	}, nil
}

func ParseEpoch(epochString string) (Epoch, error) {
	epoch, err := strconv.ParseInt(epochString, 10, 64)
	if err != nil {
		return 0, err
	}

	if epoch < 0 {
		return 0, ErrInvalidEpoch // Ensure epoch is not negative
	}

	return Epoch(epoch), nil
}

func ParseISO8601Duration(iso8601 string, until Epoch) (Epoch, error) {
	// This function should parse the ISO 8601 duration and return the start and end Epochs.
	// The implementation is omitted for brevity.
	// You can use a library like "github.com/araddon/dateparse" or implement your own parsing logic.
	duration, err := duration.Parse(iso8601)
	if err != nil {
		return 0, err
	}

	durationSeconds := math.Ceil(duration.ToTimeDuration().Seconds())
	start := until - Epoch(durationSeconds)
	if start < 0 {
		start = 0 // Ensure start is not negative
	}

	return start, nil
}

func ParseRFC3339(timestamp string) (Epoch, error) {
	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return 0, fmt.Errorf("failed to parse RFC3339 timestamp: %w", err)
	}

	return Epoch(t.Unix()), nil
}

func CurrentUnix() int64 {
	return time.Now().Unix()
}
