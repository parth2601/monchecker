package power

type PowerStats struct {
	// Empty struct since we can't reliably detect power variations
}

func ReadPowerStats() (*PowerStats, error) {
	return &PowerStats{}, nil
}

func (p *PowerStats) String() string {
	return ""
}
