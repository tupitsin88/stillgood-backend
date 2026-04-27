package geo

func ValidLatitude(value float64) bool {
	return value >= -90 && value <= 90
}

func ValidLongitude(value float64) bool {
	return value >= -180 && value <= 180
}

func ValidRadiusMeters(value int) bool {
	return value > 0
}
