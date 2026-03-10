package seeder

// InferProvince returns a Canadian province/territory code from lat/lng coordinates
// using simple bounding-box heuristics. Returns empty string if no match.
func InferProvince(lat, lng float64) string {
	for _, box := range provinceBoundingBoxes {
		if lat >= box.minLat && lat <= box.maxLat &&
			lng >= box.minLng && lng <= box.maxLng {
			return box.province
		}
	}
	return ""
}

type boundingBox struct {
	province string
	minLat   float64
	maxLat   float64
	minLng   float64
	maxLng   float64
}

// provinceBoundingBoxes defines rough bounding boxes for Canadian provinces/territories.
// Order matters: more specific (smaller) boxes should come before larger overlapping ones.
//
//nolint:gochecknoglobals,mnd // static lookup table with geographic coordinate literals
var provinceBoundingBoxes = []boundingBox{
	// Maritime provinces (check first — smaller, overlap with QC/ON longitude range)
	{province: "NS", minLat: 43.4, maxLat: 47.1, minLng: -66.5, maxLng: -59.7},
	{province: "PE", minLat: 45.9, maxLat: 47.1, minLng: -64.5, maxLng: -61.9},
	{province: "NB", minLat: 44.6, maxLat: 48.1, minLng: -69.1, maxLng: -63.8},
	{province: "NL", minLat: 46.6, maxLat: 60.4, minLng: -67.8, maxLng: -52.6},

	// Central provinces
	{province: "QC", minLat: 45.0, maxLat: 62.6, minLng: -79.8, maxLng: -57.1},
	{province: "ON", minLat: 41.7, maxLat: 56.9, minLng: -95.2, maxLng: -74.3},

	// Prairie provinces
	{province: "MB", minLat: 49.0, maxLat: 60.0, minLng: -102.1, maxLng: -88.9},
	{province: "SK", minLat: 49.0, maxLat: 60.0, minLng: -110.0, maxLng: -101.4},
	{province: "AB", minLat: 49.0, maxLat: 60.0, minLng: -120.0, maxLng: -110.0},

	// West coast
	{province: "BC", minLat: 48.3, maxLat: 60.0, minLng: -139.1, maxLng: -114.0},

	// Territories
	{province: "YT", minLat: 60.0, maxLat: 69.7, minLng: -141.0, maxLng: -124.0},
	{province: "NT", minLat: 60.0, maxLat: 78.8, minLng: -136.5, maxLng: -102.0},
	{province: "NU", minLat: 51.7, maxLat: 83.1, minLng: -120.0, maxLng: -61.0},
}
