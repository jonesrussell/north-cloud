package ingestor

import "github.com/jonesrussell/north-cloud/rfp-ingestor/internal/parser"

// NormalizeProvinceForTest exports NormalizeProvince for black-box tests.
var NormalizeProvinceForTest = parser.NormalizeProvince

// DeriveCategoriesForTest exports DeriveCategories for black-box tests.
var DeriveCategoriesForTest = parser.DeriveCategories
