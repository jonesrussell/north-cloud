package classifier

import (
	"regexp"
	"strings"
)

// Indigenous relevance constants.
const (
	indigenousRelevanceCore       = "core_indigenous"
	indigenousRelevancePeripheral = "peripheral_indigenous"
	indigenousRelevanceNot        = "not_indigenous"
)

// Indigenous category slugs — 10 global categories.
const (
	indigenousCategoryCulture     = "culture"
	indigenousCategoryLanguage    = "language"
	indigenousCategoryLandRights  = "land_rights"
	indigenousCategoryEnvironment = "environment"
	indigenousCategorySovereignty = "sovereignty"
	indigenousCategoryEducation   = "education"
	indigenousCategoryHealth      = "health"
	indigenousCategoryJustice     = "justice"
	indigenousCategoryHistory     = "history"
	indigenousCategoryCommunity   = "community"
)

// indigenousCategoryCount is the total number of canonical indigenous categories.
const indigenousCategoryCount = 10

// IndigenousCategories lists all valid indigenous category slugs.
var IndigenousCategories = []string{
	indigenousCategoryCulture,
	indigenousCategoryLanguage,
	indigenousCategoryLandRights,
	indigenousCategoryEnvironment,
	indigenousCategorySovereignty,
	indigenousCategoryEducation,
	indigenousCategoryHealth,
	indigenousCategoryJustice,
	indigenousCategoryHistory,
	indigenousCategoryCommunity,
}

// Confidence scoring constants — mirrors Python ML sidecar.
const (
	indigenousConfidenceCoreBase      = 0.60
	indigenousConfidenceCorePerHit    = 0.10
	indigenousConfidenceCoreMax       = 0.95
	indigenousConfidencePeriphBase    = 0.55
	indigenousConfidenceCatBonusPer   = 0.03
	indigenousConfidenceCatBonusMax   = 0.10
	indigenousConfidenceNotIndigenous = 0.60
)

type indigenousRuleResult struct {
	relevance  string
	confidence float64
}

// indigenousCorePatterns are strong multilingual signals for indigenous content.
var indigenousCorePatterns = []*regexp.Regexp{
	// English (Canada / North America)
	regexp.MustCompile(`(?i)\b(anishinaabe|anishinaabemowin|ojibwe|ojibwa|chippewa)\b`),
	regexp.MustCompile(`(?i)\b(first nations|indigenous peoples|indigenous community)\b`),
	regexp.MustCompile(`(?i)\b(m[eé]tis|metis nation)\b`),
	regexp.MustCompile(`(?i)\b(inuit|inuk)\b`),
	regexp.MustCompile(`(?i)\b(residential school|treaty rights|land rights|aboriginal)\b`),
	regexp.MustCompile(`(?i)\b(seven grandfathers|midewiwin|grand council)\b`),
	// English (Oceania)
	regexp.MustCompile(`(?i)\b(m[aā]ori|iwi|hap[uū]|wh[aā]nau)\b`),
	regexp.MustCompile(`(?i)\b(aboriginal australian|torres strait islander)\b`),
	// English (US / Hawaii)
	regexp.MustCompile(`(?i)\b(native hawaiian|tribal sovereignty|tribal nation)\b`),
	// English (Nordic)
	regexp.MustCompile(`(?i)\b(sami people|sámi|saami)\b`),
	// Spanish
	regexp.MustCompile(`(?i)\b(pueblos ind[ií]genas|comunidad ind[ií]gena)\b`),
	regexp.MustCompile(`(?i)\b(territorio ancestral|derechos ind[ií]genas)\b`),
	// French
	regexp.MustCompile(`(?i)\b(peuples autochtones|premi[eè]res nations)\b`),
	regexp.MustCompile(`(?i)\b(droits autochtones|communaut[eé] autochtone)\b`),
	// Portuguese
	regexp.MustCompile(`(?i)\b(povos ind[ií]genas|terra ind[ií]gena|demarca[cç][aã]o)\b`),
	// Nordic (Sami)
	regexp.MustCompile(`(?i)\b(samefolket|urfolk|samisk|s[aá]pmi)\b`),
	regexp.MustCompile(`(?i)\b(alkuper[aä]iskansa|ursprungsfolk)\b`),
	// Te Reo Māori
	regexp.MustCompile(`(?i)\b(tangata whenua|te tiriti|mana whenua)\b`),
	// Japanese (Ainu)
	regexp.MustCompile(`(アイヌ|先住民族|アイヌ民族)`),
}

// indigenousPeripheralPatterns are weaker multilingual signals.
var indigenousPeripheralPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(indigenous|native american|first nation)\b`),
	regexp.MustCompile(`(?i)\b(reconciliation|truth and reconciliation)\b`),
	regexp.MustCompile(`(?i)\b(reserve|reservation)\b`),
	regexp.MustCompile(`(?i)\b(autochtone?)\b`),
	regexp.MustCompile(`(?i)\b(ind[ií]gena)\b`),
}

const indigenousRuleMaxBodyChars = 500

// indigenousCategoryKeywords maps each category to multilingual keyword lists.
var indigenousCategoryKeywords = map[string][]string{
	indigenousCategoryCulture: {
		"culture", "ceremony", "powwow", "potlatch", "sweat lodge", "corroboree",
		"haka", "dreamtime", "totem", "regalia", "storytelling", "sacred",
		"cultura", "ceremonia", "ritual",
		"cérémonie", "tradition", "rituel",
		"cerimônia",
		"kultur", "ceremoni",
		"tikanga", "whakairo", "kapa haka",
		"文化", "儀式", "伝統",
	},
	indigenousCategoryLanguage: {
		"language", "anishinaabemowin", "indigenous language", "cree", "inuktitut",
		"te reo", "immersion", "language revitalization",
		"lengua indígena", "idioma",
		"langue autochtone",
		"língua indígena",
		"språk", "modersmål", "samiska",
		"reo", "te reo māori", "kōrero",
		"言語", "アイヌ語", "母語",
	},
	indigenousCategoryLandRights: {
		"land rights", "territory", "reserve", "reservation", "land claim",
		"land back", "native title", "dispossession",
		"territorio ancestral", "derechos territoriales", "tierras indígenas",
		"droits fonciers", "revendication territoriale",
		"terra indígena", "demarcação", "território",
		"markrättigheter", "renbetesland",
		"whenua", "mana whenua", "raupatu",
		"土地権利", "領土",
	},
	indigenousCategoryEnvironment: {
		"environment", "climate", "water rights", "pipeline", "deforestation",
		"conservation", "sacred site", "ecological",
		"medio ambiente", "deforestación", "recursos naturales",
		"environnement", "changement climatique",
		"meio ambiente", "desmatamento", "conservação",
		"miljö", "klimat", "naturresurser",
		"taiao", "kaitiakitanga", "wai",
		"環境", "気候", "自然保護",
	},
	indigenousCategorySovereignty: {
		"sovereignty", "self-determination", "self-governance", "treaty",
		"governance", "band council", "grand council", "nation-to-nation",
		"soberanía", "autodeterminación", "autogobierno",
		"souveraineté", "autodétermination", "gouvernance",
		"soberania", "autodeterminação", "governança",
		"suveränitet", "självbestämmande",
		"tino rangatiratanga", "mana motuhake",
		"主権", "自決権",
	},
	indigenousCategoryEducation: {
		"education", "residential school", "indigenous education",
		"boarding school", "curriculum", "scholarship",
		"educación", "escuela", "currículo indígena",
		"éducation", "pensionnat", "école autochtone",
		"educação", "escola indígena",
		"utbildning", "skola", "sameskola",
		"mātauranga", "kura", "wānanga",
		"教育", "学校",
	},
	indigenousCategoryHealth: {
		"health", "indigenous health", "traditional medicine",
		"mental health", "healing", "wellness",
		"salud indígena", "medicina tradicional", //nolint:misspell // Spanish word, not English
		"santé autochtone", "médecine traditionnelle",
		"saúde indígena",
		"hälsa", "traditionell medicin",
		"hauora", "rongoā",
		"健康", "伝統医療",
	},
	indigenousCategoryJustice: {
		"justice", "missing and murdered", "incarceration", "police",
		"mmiwg", "inquiry", "legal rights", "discrimination",
		"justicia", "discriminación", "derechos legales",
		"justice autochtone", "enquête",
		"justiça", "discriminação", "direitos",
		"rättvisa", "diskriminering",
		"ture", "manatika",
		"正義", "差別",
	},
	indigenousCategoryHistory: {
		"history", "colonial", "colonization", "decolonization",
		"genocide", "assimilation",
		"historia", "colonización", "descolonización",
		"histoire", "colonisation", "décolonisation",
		"história", "colonização", "descolonização",
		"kolonisering",
		"hītori", "whakapapa",
		"歴史", "植民地",
	},
	indigenousCategoryCommunity: {
		"community", "elders", "youth", "gathering", "assembly", "family",
		"comunidad", "ancianos", "juventud", "asamblea",
		"communauté", "aînés", "jeunesse", "rassemblement",
		"comunidade", "anciãos", "juventude",
		"gemenskap", "samhälle",
		"whānau", "hapū", "hui", "kaumātua",
		"コミュニティ", "長老", "集会",
	},
}

// indigenousMaxCategoryExtract limits the number of categories extracted.
const indigenousMaxCategoryExtract = 5

func classifyIndigenousByRules(title, body string) *indigenousRuleResult {
	text := title + " " + body
	if len(body) > indigenousRuleMaxBodyChars {
		text = title + " " + body[:indigenousRuleMaxBodyChars]
	}
	lower := strings.ToLower(text)

	coreHits := countPatternHits(indigenousCorePatterns, lower)
	peripheralHits := countPatternHits(indigenousPeripheralPatterns, lower)
	categoryCount := countMatchedCategories(lower)
	catBonus := float64(categoryCount) * indigenousConfidenceCatBonusPer
	if catBonus > indigenousConfidenceCatBonusMax {
		catBonus = indigenousConfidenceCatBonusMax
	}

	if coreHits >= 1 {
		confidence := indigenousConfidenceCoreBase +
			indigenousConfidenceCorePerHit*float64(coreHits) + catBonus
		if confidence > indigenousConfidenceCoreMax {
			confidence = indigenousConfidenceCoreMax
		}
		return &indigenousRuleResult{relevance: indigenousRelevanceCore, confidence: confidence}
	}
	if peripheralHits >= 1 {
		confidence := indigenousConfidencePeriphBase + catBonus
		return &indigenousRuleResult{relevance: indigenousRelevancePeripheral, confidence: confidence}
	}
	return &indigenousRuleResult{relevance: indigenousRelevanceNot, confidence: indigenousConfidenceNotIndigenous}
}

func countPatternHits(patterns []*regexp.Regexp, text string) int {
	hits := 0
	for _, p := range patterns {
		if p.MatchString(text) {
			hits++
		}
	}
	return hits
}

func countMatchedCategories(lower string) int {
	count := 0
	for _, keywords := range indigenousCategoryKeywords {
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				count++
				break
			}
		}
		if count >= indigenousMaxCategoryExtract {
			break
		}
	}
	return count
}
