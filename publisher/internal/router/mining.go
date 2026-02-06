package router

// Mining relevance constants.
const (
	MiningRelevanceNotMining  = "not_mining"
	MiningRelevancePeripheral = "peripheral_mining"
	MiningRelevanceCoreMining = "core_mining"
)

// GenerateMiningChannels returns the Redis channels for articles with mining classification.
// All core_mining and peripheral_mining articles publish to a single articles:mining channel.
func GenerateMiningChannels(article *Article) []string {
	if article.Mining == nil {
		return nil
	}

	if article.Mining.Relevance == MiningRelevanceNotMining || article.Mining.Relevance == "" {
		return nil
	}

	return []string{"articles:mining"}
}
