# M-Indigenous-Classifier: Multilingual Indigenous Classifier v3

**Status**: Implementing
**Date**: 2026-03-12
**Depends on**: D0 (global platform), D1 (region taxonomy), D2 (category taxonomy)

## Overview

Expand the Indigenous classifier from placeholder keywords to full multilingual
category-by-category keyword sets across 7 languages and 7 regions. Add
confidence scoring to both Python ML sidecar and Go rule engine. Add confidence
threshold gating in the publisher.

## Supported Languages

| # | Language | Code | Primary regions |
|---|----------|------|-----------------|
| 1 | English | en | canada, us, oceania |
| 2 | Spanish | es | latin_america |
| 3 | French | fr | canada, europe |
| 4 | Portuguese | pt | latin_america |
| 5 | Nordic (Swedish/Finnish) | sv/fi | europe |
| 6 | Te Reo Maori | mi | oceania |
| 7 | Japanese | ja | asia |

## 10 Global Categories — Keyword Sets

### 1. culture

| Language | Keywords |
|----------|----------|
| en | culture, ceremony, powwow, potlatch, sweat lodge, corroboree, haka, dreamtime, totem, regalia, storytelling, sacred |
| es | cultura, ceremonia, ritual, tradición |
| fr | culture, cérémonie, tradition, rituel |
| pt | cultura, cerimônia, ritual |
| sv/fi | kultur, ceremoni, sedvänja |
| mi | tikanga, whakairo, kapa haka |
| ja | 文化, 儀式, 伝統 |

### 2. language

| Language | Keywords |
|----------|----------|
| en | language, indigenous language, language revitalization, anishinaabemowin, cree, inuktitut, te reo, immersion |
| es | lengua indígena, idioma, revitalización lingüística |
| fr | langue autochtone, revitalisation linguistique |
| pt | língua indígena, revitalização |
| sv/fi | språk, modersmål, samiska |
| mi | reo, te reo māori, kōrero |
| ja | 言語, アイヌ語, 母語 |

### 3. land_rights

| Language | Keywords |
|----------|----------|
| en | land rights, territory, reserve, reservation, land claim, land back, native title, dispossession |
| es | territorio ancestral, derechos territoriales, tierras indígenas |
| fr | droits fonciers, territoire, revendication territoriale |
| pt | terra indígena, demarcação, território |
| sv/fi | markrättigheter, renbetesland |
| mi | whenua, mana whenua, raupatu |
| ja | 土地権利, 領土 |

### 4. environment

| Language | Keywords |
|----------|----------|
| en | environment, climate, water rights, pipeline, deforestation, conservation, sacred site, ecological |
| es | medio ambiente, deforestación, recursos naturales |
| fr | environnement, changement climatique, ressources |
| pt | meio ambiente, desmatamento, conservação |
| sv/fi | miljö, klimat, naturresurser |
| mi | taiao, kaitiakitanga, wai |
| ja | 環境, 気候, 自然保護 |

### 5. sovereignty

| Language | Keywords |
|----------|----------|
| en | sovereignty, self-determination, self-governance, treaty, governance, band council, grand council, nation-to-nation |
| es | soberanía, autodeterminación, autogobierno |
| fr | souveraineté, autodétermination, gouvernance |
| pt | soberania, autodeterminação, governança |
| sv/fi | suveränitet, självbestämmande |
| mi | tino rangatiratanga, mana motuhake |
| ja | 主権, 自決権 |

### 6. education

| Language | Keywords |
|----------|----------|
| en | education, residential school, indigenous education, boarding school, curriculum, scholarship |
| es | educación, escuela, currículo indígena |
| fr | éducation, pensionnat, école autochtone |
| pt | educação, escola indígena |
| sv/fi | utbildning, skola, sameskola |
| mi | mātauranga, kura, wānanga |
| ja | 教育, 学校 |

### 7. health

| Language | Keywords |
|----------|----------|
| en | health, indigenous health, traditional medicine, mental health, healing, wellness |
| es | salud indígena, medicina tradicional |
| fr | santé autochtone, médecine traditionnelle |
| pt | saúde indígena, medicina tradicional |
| sv/fi | hälsa, traditionell medicin |
| mi | hauora, rongoā |
| ja | 健康, 伝統医療 |

### 8. justice

| Language | Keywords |
|----------|----------|
| en | justice, missing and murdered, incarceration, police, MMIWG, inquiry, legal rights, discrimination |
| es | justicia, discriminación, derechos legales |
| fr | justice autochtone, enquête, discrimination |
| pt | justiça, discriminação, direitos |
| sv/fi | rättvisa, diskriminering |
| mi | ture, manatika |
| ja | 正義, 差別 |

### 9. history

| Language | Keywords |
|----------|----------|
| en | history, colonial, colonization, decolonization, genocide, assimilation, residential school |
| es | historia, colonización, descolonización |
| fr | histoire, colonisation, décolonisation |
| pt | história, colonização, descolonização |
| sv/fi | historia, kolonisering |
| mi | hītori, whakapapa |
| ja | 歴史, 植民地 |

### 10. community

| Language | Keywords |
|----------|----------|
| en | community, elders, youth, gathering, assembly, family |
| es | comunidad, ancianos, juventud, asamblea |
| fr | communauté, aînés, jeunesse, rassemblement |
| pt | comunidade, anciãos, juventude |
| sv/fi | gemenskap, samhälle |
| mi | whānau, hapū, hui, kaumātua |
| ja | コミュニティ, 長老, 集会 |

## Body Truncation Rules

Both Python ML sidecar and Go classifier truncate body text at 500 characters.
Title is always processed in full. This prevents long boilerplate footers from
polluting category detection.

```
effective_text = title + " " + body[:500]
```

## Confidence Scoring Model (Rule-Based)

### Python ML Sidecar

```
core_hits = count of matching CORE_PATTERNS
peripheral_hits = count of matching PERIPHERAL_PATTERNS
category_count = number of matched categories

if core_hits >= 1:
    base = 0.60
    per_hit_bonus = 0.10
    category_bonus = min(0.10, category_count * 0.03)
    confidence = min(0.95, base + per_hit_bonus * core_hits + category_bonus)

if peripheral_hits >= 1:
    confidence = 0.55 + min(0.10, category_count * 0.03)

not_indigenous:
    confidence = 0.60
```

The `language_detected` field reports the first matched language group
(en, es, fr, pt, sv, mi, ja) or "unknown" if only generic patterns matched.

### Go Classifier

Same logic, mirrored. The Go classifier counts core/peripheral pattern hits
and computes confidence identically to ensure consistent scoring.

### Publisher Confidence Threshold

The publisher applies a confidence threshold of **0.35** to the
`indigenous.final_confidence` field. Content below this threshold is not
routed to any indigenous channel. This gates out very low-confidence
classifications that would clutter category feeds.

## Testing Matrix

### Language x Relevance

| Language | Core test | Peripheral test | Not-indigenous test |
|----------|-----------|-----------------|---------------------|
| English | Anishinaabe, Maori, etc. | "Indigenous art" | "Weather forecast" |
| Spanish | "Pueblos indígenas" | "indígena" | "El clima" |
| French | "Peuples autochtones" | "autochtone" | "La météo" |
| Portuguese | "Povos indígenas" | "indígena" | "O tempo" |
| Nordic | "Samefolket" | — | "Vädret" |
| Te Reo | "Tangata whenua" | — | — |
| Japanese | "アイヌ民族" | — | — |

### Category x Language

Each category should have at least one test in English plus one in a
non-English language to verify multilingual extraction.

### Special Cases

- Mixed-language content (English title + Spanish body)
- Low-confidence edge cases (body-only match, no title match)
- Non-indigenous false positives (generic "reserve" in banking context)
- Multiple categories in same document
- Body truncation boundary test

## Region-Specific Keyword Considerations

| Region | Notes |
|--------|-------|
| canada | Default region; richest English/French keyword coverage |
| us | "tribal sovereignty", "native american", "reservation" |
| latin_america | Spanish/Portuguese dominate; "pueblos indígenas" |
| oceania | Maori/Aboriginal terms; Te Reo Maori keywords |
| europe | Nordic/Sami keywords; "samefolket", "urfolk" |
| asia | Japanese Ainu terms; limited to well-established terms |
| africa | Future expansion — minimal keyword coverage for now |

## Future Extensions

- **ML model v3**: Replace rule-based patterns with a fine-tuned
  multilingual transformer (mBERT or XLM-R). Rule engine becomes fallback.
- **People-level tagging**: Named entity recognition for Indigenous leaders,
  organizations, and communities. Requires NER model integration.
- **Region auto-detection**: Infer region from content language and geographic
  entity mentions rather than requiring source-level configuration.
- **Confidence calibration**: Use labeled samples to calibrate confidence
  scores via Platt scaling or isotonic regression.
