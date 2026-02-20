package router

import "strings"

// Mining relevance constants.
const (
	MiningRelevanceNotMining  = "not_mining"
	MiningRelevancePeripheral = "peripheral_mining"
	MiningRelevanceCoreMining = "core_mining"
)

// Mining location constants.
const (
	MiningLocationLocalCanada    = "local_canada"
	MiningLocationNationalCanada = "national_canada"
	MiningLocationInternational  = "international"
)

// Mining stage to skip when generating stage channels.
const miningStageUnspecified = "unspecified"

// GenerateMiningChannels returns the Redis channels for articles with mining classification.
// Channels generated:
//   - articles:mining          (catch-all: all core + peripheral)
//   - mining:core              (core_mining only)
//   - mining:peripheral        (peripheral_mining only)
//   - mining:commodity:{slug}  (per commodity, e.g. mining:commodity:gold)
//   - mining:stage:{stage}     (per mining stage, skips "unspecified")
//   - mining:canada            (local_canada or national_canada)
//   - mining:international     (international)
func GenerateMiningChannels(article *Article) []string {
	if article.Mining == nil {
		return nil
	}

	rel := article.Mining.Relevance
	if rel == MiningRelevanceNotMining || rel == "" {
		return nil
	}

	channels := []string{"articles:mining"}

	channels = appendRelevanceChannel(channels, rel)
	channels = appendCommodityChannels(channels, article.Mining.Commodities)
	channels = appendStageChannel(channels, article.Mining.MiningStage)
	channels = appendMiningLocationChannel(channels, article.Mining.Location)

	return channels
}

// appendRelevanceChannel adds mining:core or mining:peripheral based on relevance.
func appendRelevanceChannel(channels []string, relevance string) []string {
	switch relevance {
	case MiningRelevanceCoreMining:
		return append(channels, "mining:core")
	case MiningRelevancePeripheral:
		return append(channels, "mining:peripheral")
	default:
		return channels
	}
}

// appendCommodityChannels adds mining:commodity:{slug} for each commodity.
// Underscores are converted to hyphens (e.g. iron_ore → iron-ore).
func appendCommodityChannels(channels, commodities []string) []string {
	for _, c := range commodities {
		slug := strings.ToLower(strings.ReplaceAll(c, "_", "-"))
		if slug != "" {
			channels = append(channels, "mining:commodity:"+slug)
		}
	}

	return channels
}

// appendStageChannel adds mining:stage:{stage} if the stage is specified.
func appendStageChannel(channels []string, stage string) []string {
	if stage == "" || stage == miningStageUnspecified {
		return channels
	}

	return append(channels, "mining:stage:"+strings.ToLower(stage))
}

// appendMiningLocationChannel adds mining:canada or mining:international.
func appendMiningLocationChannel(channels []string, location string) []string {
	switch location {
	case MiningLocationLocalCanada, MiningLocationNationalCanada:
		return append(channels, "mining:canada")
	case MiningLocationInternational:
		return append(channels, "mining:international")
	default:
		return channels
	}
}

// MiningDomain routes mining-classified articles to mining:* channels.
type MiningDomain struct{}

// NewMiningDomain creates a MiningDomain.
func NewMiningDomain() *MiningDomain { return &MiningDomain{} }

// Name returns the domain identifier.
func (d *MiningDomain) Name() string { return "mining" }

// Routes returns mining channels for the article. Returns nil if the article
// is not mining-classified. Delegates to GenerateMiningChannels.
func (d *MiningDomain) Routes(a *Article) []ChannelRoute {
	return channelRoutesFromSlice(GenerateMiningChannels(a))
}
