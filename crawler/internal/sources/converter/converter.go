// Package converter provides generic type conversion utilities for source configurations.
// It uses the mapstructure library to efficiently copy fields between similar struct types.
package converter

import (
	"github.com/mitchellh/mapstructure"
)

// Convert uses mapstructure to convert between similar struct types.
// This eliminates the need for manual field-by-field copying.
//
// Example:
//
//	var result types.ArticleSelectors
//	if err := Convert(sourceSelectors, &result); err != nil {
//	    return err
//	}
//
// The function supports conversion between types with matching field names,
// which is perfect for our use case where we're converting between
// configtypes, loader types, and internal types that have identical fields.
func Convert(src, dst any) error {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName: "json", // Use JSON tags for field matching
		Result:  dst,
		// WeaklyTypedInput allows for more flexible type conversion
		WeaklyTypedInput: true,
		// ZeroFields ensures destination fields are zeroed before decoding
		ZeroFields: true,
	})
	if err != nil {
		return err
	}

	return decoder.Decode(src)
}

// ConvertValue is a generic helper that returns the converted value.
// It's useful when you want to convert and return in one step.
//
// Example:
//
//	result, err := ConvertValue[types.ArticleSelectors](sourceSelectors)
//	if err != nil {
//	    return types.ArticleSelectors{}, err
//	}
//
// Note: Requires Go 1.18+ for generics support.
func ConvertValue[T any](src any) (T, error) {
	var result T
	err := Convert(src, &result)
	return result, err
}
