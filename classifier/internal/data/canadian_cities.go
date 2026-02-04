// classifier/internal/data/canadian_cities.go
package data

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// CityInfo contains metadata about a Canadian city.
type CityInfo struct {
	Canonical string // Normalized slug form
	Province  string // Two-letter province code
}

// canadianCities maps normalized city names to their info.
// This is a curated list of major Canadian cities and municipalities.
// Source: Statistics Canada Census Metropolitan Areas + major municipalities.
var canadianCities = map[string]CityInfo{
	// Ontario
	"toronto":          {Canonical: "toronto", Province: "ON"},
	"ottawa":           {Canonical: "ottawa", Province: "ON"},
	"mississauga":      {Canonical: "mississauga", Province: "ON"},
	"brampton":         {Canonical: "brampton", Province: "ON"},
	"hamilton":         {Canonical: "hamilton", Province: "ON"},
	"london":           {Canonical: "london", Province: "ON"},
	"markham":          {Canonical: "markham", Province: "ON"},
	"vaughan":          {Canonical: "vaughan", Province: "ON"},
	"kitchener":        {Canonical: "kitchener", Province: "ON"},
	"windsor":          {Canonical: "windsor", Province: "ON"},
	"sudbury":          {Canonical: "sudbury", Province: "ON"},
	"greater sudbury":  {Canonical: "sudbury", Province: "ON"},
	"thunder bay":      {Canonical: "thunder-bay", Province: "ON"},
	"north bay":        {Canonical: "north-bay", Province: "ON"},
	"sault ste marie":  {Canonical: "sault-ste-marie", Province: "ON"},
	"sault ste. marie": {Canonical: "sault-ste-marie", Province: "ON"},
	"timmins":          {Canonical: "timmins", Province: "ON"},
	"peterborough":     {Canonical: "peterborough", Province: "ON"},
	"kingston":         {Canonical: "kingston", Province: "ON"},
	"guelph":           {Canonical: "guelph", Province: "ON"},
	"cambridge":        {Canonical: "cambridge", Province: "ON"},
	"waterloo":         {Canonical: "waterloo", Province: "ON"},
	"barrie":           {Canonical: "barrie", Province: "ON"},
	"oshawa":           {Canonical: "oshawa", Province: "ON"},
	"st catharines":    {Canonical: "st-catharines", Province: "ON"},
	"st. catharines":   {Canonical: "st-catharines", Province: "ON"},
	"niagara falls":    {Canonical: "niagara-falls", Province: "ON"},
	"welland":          {Canonical: "welland", Province: "ON"},
	"brantford":        {Canonical: "brantford", Province: "ON"},
	"sarnia":           {Canonical: "sarnia", Province: "ON"},
	"belleville":       {Canonical: "belleville", Province: "ON"},
	"cornwall":         {Canonical: "cornwall", Province: "ON"},
	"chatham":          {Canonical: "chatham", Province: "ON"},
	"orillia":          {Canonical: "orillia", Province: "ON"},
	"owen sound":       {Canonical: "owen-sound", Province: "ON"},
	"espanola":         {Canonical: "espanola", Province: "ON"},
	"elliot lake":      {Canonical: "elliot-lake", Province: "ON"},
	"kirkland lake":    {Canonical: "kirkland-lake", Province: "ON"},
	"kapuskasing":      {Canonical: "kapuskasing", Province: "ON"},
	"kenora":           {Canonical: "kenora", Province: "ON"},

	// Quebec
	"montreal":       {Canonical: "montreal", Province: "QC"},
	"quebec city":    {Canonical: "quebec-city", Province: "QC"},
	"quebec":         {Canonical: "quebec-city", Province: "QC"},
	"laval":          {Canonical: "laval", Province: "QC"},
	"gatineau":       {Canonical: "gatineau", Province: "QC"},
	"longueuil":      {Canonical: "longueuil", Province: "QC"},
	"sherbrooke":     {Canonical: "sherbrooke", Province: "QC"},
	"trois-rivieres": {Canonical: "trois-rivieres", Province: "QC"},
	"trois rivieres": {Canonical: "trois-rivieres", Province: "QC"},
	"chicoutimi":     {Canonical: "chicoutimi", Province: "QC"},
	"saguenay":       {Canonical: "saguenay", Province: "QC"},

	// British Columbia
	"vancouver":     {Canonical: "vancouver", Province: "BC"},
	"surrey":        {Canonical: "surrey", Province: "BC"},
	"burnaby":       {Canonical: "burnaby", Province: "BC"},
	"richmond":      {Canonical: "richmond", Province: "BC"},
	"victoria":      {Canonical: "victoria", Province: "BC"},
	"kelowna":       {Canonical: "kelowna", Province: "BC"},
	"abbotsford":    {Canonical: "abbotsford", Province: "BC"},
	"nanaimo":       {Canonical: "nanaimo", Province: "BC"},
	"kamloops":      {Canonical: "kamloops", Province: "BC"},
	"prince george": {Canonical: "prince-george", Province: "BC"},
	"chilliwack":    {Canonical: "chilliwack", Province: "BC"},
	"vernon":        {Canonical: "vernon", Province: "BC"},
	"courtenay":     {Canonical: "courtenay", Province: "BC"},

	// Alberta
	"calgary":        {Canonical: "calgary", Province: "AB"},
	"edmonton":       {Canonical: "edmonton", Province: "AB"},
	"red deer":       {Canonical: "red-deer", Province: "AB"},
	"lethbridge":     {Canonical: "lethbridge", Province: "AB"},
	"medicine hat":   {Canonical: "medicine-hat", Province: "AB"},
	"grande prairie": {Canonical: "grande-prairie", Province: "AB"},
	"fort mcmurray":  {Canonical: "fort-mcmurray", Province: "AB"},

	// Manitoba
	"winnipeg":  {Canonical: "winnipeg", Province: "MB"},
	"brandon":   {Canonical: "brandon", Province: "MB"},
	"steinbach": {Canonical: "steinbach", Province: "MB"},
	"thompson":  {Canonical: "thompson", Province: "MB"},

	// Saskatchewan
	"saskatoon":     {Canonical: "saskatoon", Province: "SK"},
	"regina":        {Canonical: "regina", Province: "SK"},
	"prince albert": {Canonical: "prince-albert", Province: "SK"},
	"moose jaw":     {Canonical: "moose-jaw", Province: "SK"},

	// Nova Scotia
	"halifax":   {Canonical: "halifax", Province: "NS"},
	"dartmouth": {Canonical: "dartmouth", Province: "NS"},
	"sydney":    {Canonical: "sydney", Province: "NS"},
	"truro":     {Canonical: "truro", Province: "NS"},

	// New Brunswick
	"saint john":  {Canonical: "saint-john", Province: "NB"},
	"moncton":     {Canonical: "moncton", Province: "NB"},
	"fredericton": {Canonical: "fredericton", Province: "NB"},

	// Newfoundland and Labrador
	"st johns":     {Canonical: "st-johns", Province: "NL"},
	"st. johns":    {Canonical: "st-johns", Province: "NL"},
	"st. john's":   {Canonical: "st-johns", Province: "NL"},
	"corner brook": {Canonical: "corner-brook", Province: "NL"},
	"mount pearl":  {Canonical: "mount-pearl", Province: "NL"},

	// Prince Edward Island
	"charlottetown": {Canonical: "charlottetown", Province: "PE"},
	"summerside":    {Canonical: "summerside", Province: "PE"},

	// Territories
	"whitehorse":  {Canonical: "whitehorse", Province: "YT"},
	"yellowknife": {Canonical: "yellowknife", Province: "NT"},
	"iqaluit":     {Canonical: "iqaluit", Province: "NU"},
}

// prefixesToRemove are common prefixes that should be stripped for normalization.
var prefixesToRemove = []string{
	"greater ",
	"city of ",
	"town of ",
	"municipality of ",
	"regional municipality of ",
}

// IsValidCanadianCity checks if the given city name is in the validated Canadian cities list.
func IsValidCanadianCity(city string) bool {
	if city == "" {
		return false
	}
	normalized := normalizeForLookup(city)
	_, ok := canadianCities[normalized]
	return ok
}

// NormalizeCityName returns the canonical slug form of a city name.
// Returns empty string if the city is not found.
func NormalizeCityName(city string) string {
	if city == "" {
		return ""
	}
	normalized := normalizeForLookup(city)
	if info, ok := canadianCities[normalized]; ok {
		return info.Canonical
	}
	// If not found, return a best-effort slug
	return toSlug(city)
}

// GetProvinceForCity returns the province code for a Canadian city.
func GetProvinceForCity(city string) (string, bool) {
	if city == "" {
		return "", false
	}
	normalized := normalizeForLookup(city)
	if info, ok := canadianCities[normalized]; ok {
		return info.Province, true
	}
	return "", false
}

// normalizeForLookup prepares a city name for map lookup.
func normalizeForLookup(city string) string {
	s := strings.ToLower(strings.TrimSpace(city))

	// Remove common prefixes
	for _, prefix := range prefixesToRemove {
		if after, found := strings.CutPrefix(s, prefix); found {
			s = after
			break
		}
	}

	// Remove accents for lookup
	s = removeAccents(s)

	return s
}

// toSlug converts a city name to a URL-safe slug.
func toSlug(city string) string {
	s := strings.ToLower(strings.TrimSpace(city))

	// Remove common prefixes
	for _, prefix := range prefixesToRemove {
		if after, found := strings.CutPrefix(s, prefix); found {
			s = after
			break
		}
	}

	// Remove accents
	s = removeAccents(s)

	// Replace spaces and special chars with hyphens
	s = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(s, "-")

	// Clean up multiple hyphens and trim
	s = regexp.MustCompile(`-+`).ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")

	return s
}

// removeAccents strips diacritical marks from a string.
func removeAccents(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	result, _, _ := transform.String(t, s)
	return result
}
