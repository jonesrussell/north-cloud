# StreetCode.net Classifier Taxonomy

**Date**: 2026-02-03
**Status**: Approved for Implementation
**Purpose**: Define the classification schema for street crime content on StreetCode.net

---

## Overview

This taxonomy supports two primary goals:

1. **High-precision homepage feed** — Only core street crime content appears on the main page
2. **Granular category pages** — Consistent, unambiguous crime type categorization

### Design Principles

- Precision > recall for homepage (rather miss a story than show irrelevant content)
- Recall > precision for category pages (acceptable to include peripheral items)
- No over-tagging — only assign crime labels when evidence is strong
- Hybrid rule + ML approach — rules for precision, ML for coverage

---

## Label 1: Street Crime Relevance

**Purpose**: Gatekeeper for homepage display
**Type**: Single-label, mutually exclusive (exactly one value per article)

| Value | Definition | Homepage? |
|-------|------------|-----------|
| `core_street_crime` | Article's primary subject is a specific criminal incident, arrest, investigation, or court proceeding for a street-level offense. Must name a victim, suspect, location, or specific event. | Yes |
| `peripheral_crime` | Crime is mentioned but not the main focus. Includes: policy debates, statistics, opinion pieces, human interest stories, general safety warnings. | No |
| `not_crime` | No meaningful crime content. | Excluded |

### Decision Rules

| # | Condition | Result |
|---|-----------|--------|
| 1 | Article's lead describes a **specific criminal incident** (even without arrest) | `core_street_crime` |
| 2 | **Court/sentencing story** directly tied to a street-level offense | `core_street_crime` |
| 3 | **Public safety alert** for a specific, time-bound incident (armed suspect at large) | `core_street_crime` |
| 4 | **Multi-topic article** where crime is a major section with concrete details | `core_street_crime` |
| 5 | General **public safety warning** without specific incident | `peripheral_crime` |
| 6 | **Policy, statistics, opinion** about crime | `peripheral_crime` |
| 7 | **Victim/community stories** without new investigation details | `peripheral_crime` |
| 8 | **Missing persons** without confirmed criminal suspicion | `peripheral_crime` |
| 9 | **Weapons possession only** (no use or threat) | `peripheral_crime` |
| 10 | **Overdose** without trafficking/dealing context | `not_crime` |
| 11 | **Fire** without arson investigation | `not_crime` |
| 12 | Court story about **non-crime matters** (budget lawsuits, civil cases) | `not_crime` |
| 13 | **Sexual misconduct** without criminal charges | `not_crime` or `peripheral_crime` |

### The "Lead Paragraph" Test

If the first 2-3 sentences describe a specific criminal incident → `core_street_crime`

---

## Label 2: Crime Type

**Purpose**: Route articles to category pages
**Type**: Multi-label (zero or more values per article)

| Type | Definition | Public Category |
|------|------------|-----------------|
| `violent_crime` | Physical harm or threat to persons. Murder, homicide, assault, shooting, stabbing, robbery with violence, domestic violence, sexual assault, kidnapping, hostage situations. | "Violent Crime" |
| `property_crime` | Unlawful taking or destruction of property without violence. Theft, burglary, break-and-enter, shoplifting, auto theft, vandalism, arson (default), street-level fraud. | "Property Crime" |
| `drug_crime` | Controlled substance offenses. Trafficking, dealing, major busts, possession with intent, drug labs, smuggling. Excludes personal use, overdoses without criminal context. | "Drug Crime" |
| `gang_violence` | Crimes explicitly linked to gang activity, turf disputes, gang membership. Often co-occurs with `violent_crime`. | "Gang Violence" |
| `organized_crime` | Coordinated criminal enterprises. Mafia, cartels, human trafficking rings, large-scale money laundering, RICO cases. Distinct from street-level gang activity. | "Organized Crime" |
| `criminal_justice` | Court proceedings, sentencing, police operations for street-level offenses. **Modifier only** — always paired with underlying crime type. | Internal only |
| `other_crime` | **Internal only** — ML fallback for edge cases. Animal cruelty, environmental crime, rare categories. Never shown publicly. | Internal only |

### Edge Case Rules

**Arson**:
- Default → `property_crime`
- With intent to harm people → `violent_crime`
- Linked to organized crime → `organized_crime` + `property_crime`

**Sexual Offenses**:
- Criminal charges (sexual assault, interference) → `violent_crime`
- Non-criminal misconduct (workplace, school) → `peripheral_crime` or `not_crime`

**Weapons Offenses**:
- Possession only (no use or threat) → `peripheral_crime` + `criminal_justice`
- Use or threat → `violent_crime`

**Fraud**:
- Street-level + specific suspect/arrest/incident → `property_crime`
- General scam warnings, phone scams, CRA scams → `peripheral_crime`

### Keywords by Crime Type

**violent_crime**: murder, homicide, assault, shooting, stabbing, attack, attacked, weapon, armed, gunman, shooter, beating, gang violence, drive-by, domestic violence, sexual assault, rape, kidnapping, abduction, hostage, manslaughter

**property_crime**: theft, robbery, burglary, stolen, shoplifting, larceny, break-in, breaking and entering, trespassing, vandalism, carjacking, auto theft, stolen car, stolen vehicle, vehicle theft, arson, graffiti, property damage, looting

**drug_crime**: drug, drugs, narcotics, trafficking, dealer, possession, cocaine, heroin, fentanyl, methamphetamine, meth, marijuana, cannabis, opioid, drug bust, drug ring, cartel, smuggling, drug trafficking, controlled substance

**gang_violence**: gang, gang member, gang-related, turf war, drive-by, gang shooting, gang rivalry, gang activity

**organized_crime**: organized crime, mafia, mob, cartel, crime syndicate, racketeering, extortion, money laundering, trafficking ring, human trafficking, RICO, crime family, kingpin, crime boss, criminal organization

**criminal_justice**: court, trial, conviction, sentence, sentencing, verdict, arraignment, indictment, plea deal, hearing, appeal, prison, jail, incarceration, parole, probation, warrant, arrest, arrested, investigation, detective, prosecution, prosecutor, judge, jury

---

## Label 3: Location Specificity

**Purpose**: Content ranking and relevance boosting
**Type**: Single-label (exactly one value per article)
**Visibility**: Internal only

| Value | Definition |
|-------|------------|
| `local_canada` | Incident in a specific Canadian city/region. Article names the location. |
| `national_canada` | Canada-wide story or multiple provinces. No single local focus. |
| `international` | Crime outside Canada but with Canadian relevance (victim, suspect, policy impact). |
| `not_specified` | Location unclear or not relevant. |

### Decision Rules

- Default to `local_canada` if any Canadian city/region is named
- Use `national_canada` only for explicitly multi-province or federal stories
- `international` only if article explicitly states non-Canadian location

---

## Label Visibility Summary

| Label | Public-Facing | Internal Use |
|-------|---------------|--------------|
| `street_crime_relevance` | No | Homepage gating |
| `violent_crime` | Yes — "Violent Crime" | — |
| `property_crime` | Yes — "Property Crime" | — |
| `drug_crime` | Yes — "Drug Crime" | — |
| `gang_violence` | Yes — "Gang Violence" | — |
| `organized_crime` | Yes — "Organized Crime" | — |
| `criminal_justice` | No | "Court News" filtering |
| `other_crime` | No | ML fallback |
| `location_specificity` | No | Ranking boost |

---

## Website Routing Logic

| Destination | Classifier Logic |
|-------------|------------------|
| **Homepage Feed** | `street_crime_relevance == core_street_crime` AND `content_type == article` AND confidence >= 0.7 |
| **Violent Crime** | `violent_crime` in crime_types |
| **Drug Crime** | `drug_crime` in crime_types |
| **Property Crime** | `property_crime` in crime_types |
| **Gang Violence** | `gang_violence` in crime_types |
| **Organized Crime** | `organized_crime` in crime_types |
| **Court News** | `criminal_justice` in crime_types (secondary page) |

---

## Classifier Output Schema

```json
{
  "street_crime_relevance": "core_street_crime | peripheral_crime | not_crime",
  "crime_types": ["violent_crime", "drug_crime"],
  "location_specificity": "local_canada | national_canada | international | not_specified",
  "confidence": 0.85,
  "content_type": "article | page | listing"
}
```

---

## Appendix: Synthetic Examples

### Core Street Crime

| Headline | crime_types | Reasoning |
|----------|-------------|-----------|
| "Man charged with murder after downtown stabbing" | violent_crime, criminal_justice | Specific incident, arrest, named offense |
| "Police seeking witnesses to Saturday night shooting" | violent_crime | Active investigation, specific event |
| "Three arrested in $500K fentanyl trafficking bust" | drug_crime, criminal_justice | Major bust, arrests |
| "Gang member sentenced for drive-by shooting" | violent_crime, gang_violence, criminal_justice | Sentencing for street offense |
| "Armed robbery at downtown convenience store" | violent_crime, property_crime | Violence + theft |

### Peripheral Crime

| Headline | Reasoning |
|----------|-----------|
| "Mayor calls for crackdown on property crime" | Policy, no specific incident |
| "How to protect your car from theft this winter" | Safety advice |
| "Crime down 10% in Sudbury this year" | Statistics only |
| "Family remembers shooting victim one year later" | Human interest, no new details |
| "Police warn of spike in catalytic converter thefts" | General warning, no specific incident |
| "Man charged with weapons possession after traffic stop" | Weapons possession only, no use/threat |

### Not Crime

| Headline | Reasoning |
|----------|-----------|
| "Sudbury Music Festival announces 2026 lineup" | No crime connection |
| "City council debates police budget" | Administrative, not crime |
| "Fire destroys downtown warehouse" | No arson investigation mentioned |
| "Overdose deaths rise in northern Ontario" | Health issue, no criminal context |
| "Teacher suspended for inappropriate conduct" | Non-criminal misconduct |

---

## Version History

| Date | Change |
|------|--------|
| 2026-02-03 | Initial taxonomy design |
