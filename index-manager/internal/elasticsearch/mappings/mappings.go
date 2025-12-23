package mappings

import (
	"encoding/json"
)

// BaseSettings defines common index-level settings
type BaseSettings struct {
	NumberOfShards   int `json:"number_of_shards"`
	NumberOfReplicas int `json:"number_of_replicas"`
}

// DefaultSettings returns the default index settings
func DefaultSettings() BaseSettings {
	return BaseSettings{
		NumberOfShards:   1,
		NumberOfReplicas: 1,
	}
}

// ToMap converts a mapping to a map[string]interface{} for Elasticsearch
func ToMap(mapping interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(mapping)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return result, nil
}
