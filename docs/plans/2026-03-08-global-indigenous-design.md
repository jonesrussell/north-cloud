# Global Indigenous Content Platform — Design

**Date:** 2026-03-08
**Status:** Approved
**Goal:** Expand NorthCloud's indigenous content pipeline from Canadian-only to global coverage across all regions, languages, and peoples.

---

## Taxonomy

### Region Tags (source-level metadata)

Applied at the source-manager level. Each source gets one `indigenous_region` tag. Determines publisher routing.

| Tag | Coverage |
|-----|----------|
| `canada` | First Nations, Métis, Inuit |
| `us` | Native American, Alaska Native, Native Hawaiian |
| `latin_america` | Maya, Quechua, Mapuche, Guaraní, Wayuu, Amazonian peoples |
| `oceania` | Aboriginal Australian, Torres Strait Islander, Māori, Pacific Islander |
| `europe` | Sámi, Basque (indigenous context), Roma (indigenous context) |
| `asia` | Ainu, Adivasi, Tibetan, Hmong, indigenous Taiwanese |
| `africa` | San, Maasai, Pygmy/Batwa, Amazigh/Berber, Ogiek |

People-level tags (`indigenous_people: maori`, `indigenous_people: sami`) are a future addition — region is the correct first dimension.

### Content Categories (classifier-level)

Replaces the current Canada-centric 6-category set with 10 globally applicable categories aligned with UNDRIP themes.

| Category | Description |
|----------|-------------|
| `culture` | Ceremonies, art, music, dance, storytelling, traditional practices |
| `language` | Language revitalization, immersion programs, dictionaries, media in indigenous languages |
| `land_rights` | Territory, reserves, land claims, land back, sacred sites |
| `environment` | Environmental stewardship, climate impact on indigenous lands, traditional ecological knowledge |
| `sovereignty` | Self-governance, tribal sovereignty, treaty rights, self-determination |
| `education` | Schools, universities, indigenous curricula, residential school legacy |
| `health` | Indigenous health disparities, traditional medicine, mental health, MMIWG |
| `justice` | Incarceration, policing, legal rights, indigenous courts, reparations |
| `history` | Historical events, colonization, resistance, oral history |
| `community` | Community news, events, celebrations, economic development |

### Backward Compatibility

The existing `anishinaabe` category becomes a sub-tag of `culture` + `canada` region. Existing `content:indigenous` channel continues to carry all indigenous content. New channels are additive:

```
content:indigenous                        (all — existing, unchanged)
indigenous:region:canada                  (existing Canadian content)
indigenous:region:oceania                 (new)
indigenous:region:latin_america           (new)
indigenous:region:{region}                (new, per region)
indigenous:category:sovereignty           (new, cross-region)
indigenous:category:{category}            (new, per category)
```

---

## Architecture Changes

### Source-Manager

Add `indigenous_region` column to `sources` table:

```sql
ALTER TABLE sources ADD COLUMN indigenous_region TEXT;
```

Nullable — only set for indigenous sources. Values: `canada`, `us`, `latin_america`, `oceania`, `europe`, `asia`, `africa`.

The API source struct gains:
```go
IndigenousRegion *string `db:"indigenous_region" json:"indigenous_region,omitempty"`
```

### Classifier — Multilingual Pattern Expansion

The indigenous classifier currently has English-only patterns. Expansion per language group:

**English** (existing — works for canada, us, oceania, africa):
- Current patterns already cover: `first nations`, `indigenous`, `treaty rights`, etc.
- Add: `aboriginal`, `māori`, `iwi`, `native hawaiian`, `tribal sovereignty`

**Spanish** (latin_america):
- `pueblos indígenas`, `comunidad indígena`, `territorio ancestral`
- `derechos indígenas`, `lengua indígena`, `autodeterminación`
- `pueblos originarios`, `nación indígena`, `tierra sagrada`

**French** (canada Francophone, africa):
- `peuples autochtones`, `premières nations`, `droits autochtones`
- `territoire ancestral`, `réconciliation`, `communauté autochtone`

**Portuguese** (brazil):
- `povos indígenas`, `terra indígena`, `demarcação`
- `comunidade indígena`, `direitos indígenas`, `aldeia`

**Norwegian/Swedish/Finnish** (europe — Sámi):
- `samefolket`, `urfolk`, `samisk`, `sápmi`
- `alkuperäiskansa` (Finnish), `ursprungsfolk` (Swedish)

**Te Reo Māori** (oceania):
- `tangata whenua`, `te tiriti`, `iwi`, `hapū`, `whānau`
- `mana whenua`, `kaitiakitanga`, `tikanga`

**Japanese** (asia — Ainu):
- `アイヌ`, `先住民族`, `アイヌ民族`

Each language group gets its own pattern file in the ML sidecar and Go classifier.

### Publisher — Region Routing

Add `IndigenousRegionDomain` to the routing pipeline (or extend existing `IndigenousDomain`):

```go
// In addition to existing category channels:
if source.IndigenousRegion != "" {
    channels = append(channels, "indigenous:region:"+source.IndigenousRegion)
}
```

Region comes from source metadata (not classifier), so it's deterministic and free.

### Crawler — Source Config

The crawler's `apiclient/types.go` gains `IndigenousRegion` from the source-manager API response. No crawler logic changes needed — it just passes through to raw_content metadata.

---

## Source Research by Region

### Canada (existing — 28 loaded, ~28 more from JSON)
Remaining from `anishinaabe-sources-data.json`: Wawatay News, Turtle Island News, Nunatsiaq News, Nunavut News, Two Row Times, Eastern Door, Southern Chiefs Organization, etc.

### United States
- Indian Country Today (indiancountrytoday.com)
- Native News Online (nativenewsonline.net)
- Tribal Business News (tribalbusinessnews.com)
- Navajo Times (navajotimes.com)
- Cherokee Phoenix (cherokeephoenix.org)
- Lakota Times (lakotatimes.com)
- High Country News — Indigenous Affairs
- Cronkite News — Indian Country
- ICT (formerly Indian Country Today Media Network)

### Latin America
- Servindi (servindi.org) — Peru, pan-indigenous, Spanish
- Mongabay Latam — indigenous coverage, Spanish/Portuguese
- Territorio Indígena (territorioindigenaygobernanza.com)
- IWGIA Latin America reports
- Survival International — Americas section
- Cultural Survival (culturalsurvival.org)

### Oceania
- NITV (National Indigenous Television, Australia)
- The Guardian — Indigenous Australians section
- Māori Television (maoritelevision.com)
- Te Ao Māori News (teaomaori.news)
- Stuff NZ — Pou Tiaki (indigenous section)
- Pacific Islands News Association (pina.com.fj)
- RNZ — Te Manu Korihi

### Europe
- Sámi Radio (NRK Sápmi — nrk.no/sapmi)
- Yle Sápmi (Finnish Sámi news)
- SVT Sápmi (Swedish)
- The Barents Observer — Sámi coverage
- IWGIA Europe reports

### Asia
- Survival International — Asia section
- IWGIA Asia reports
- Ainu Association of Hokkaido
- Adivasi Lives Matter (India)
- AMAN (Aliansi Masyarakat Adat Nusantara — Indonesia)

### Africa
- IWGIA Africa reports
- Survival International — Africa section
- Natural Justice (naturaljustice.org)
- Forest Peoples Programme

---

## Implementation Sequence

### Milestone: Indigenous Classifier Expansion (do first)

1. Add `indigenous_region` column to source-manager
2. Backfill existing 28 indigenous sources with `region: canada`
3. Expand English patterns for US, Oceania, Africa
4. Add Spanish patterns for Latin America
5. Add French patterns for Canada Francophone + Africa
6. Add Portuguese patterns for Brazil
7. Add Nordic patterns for Sámi
8. Add Te Reo Māori patterns for Oceania
9. Add Japanese patterns for Ainu
10. Expand category set from 6 → 10
11. Add publisher region routing
12. Test each language group with sample content

### Milestone: Global Indigenous Source Onboarding

1. Load remaining ~28 Canadian sources from JSON
2. Research and add US sources (~15-20)
3. Research and add Latin American sources (~10-15)
4. Research and add Oceania sources (~10-15)
5. Research and add European sources (~5-10)
6. Research and add Asian sources (~5-10)
7. Research and add African sources (~5-10)
8. Validate classification accuracy per region
9. Create Grafana dashboard for indigenous content by region

### Milestone: Indigenous Content Quality (future)

1. Monitor extraction quality per region
2. Add CMS templates for common indigenous media platforms
3. Auto-detect indigenous content from non-indigenous sources
4. Add people-level tags where warranted
5. Build indigenous content search vertical

---

## Success Criteria

- Indigenous sources from all 7 regions actively crawled
- Classifier correctly identifies indigenous content in ≥4 languages
- Publisher routes to region-specific channels
- `content:indigenous` channel carries global indigenous content
- Extraction success rate ≥50% across indigenous sources
- Zero false positives from non-indigenous mainstream sources
