# StreetCode Classifier Rule Set

**Date**: 2026-02-03
**Purpose**: High-precision keyword rules for street crime classification
**Usage**: Bootstrap training data + fallback when ML confidence is low

---

## Rule Priority

Rules are evaluated in order. First match wins for `street_crime_relevance`.
Crime types are multi-label (all matching rules apply).

---

## 1. Exclusion Patterns (Check First)

These patterns indicate `not_crime` regardless of other keywords:

### Content Type Exclusions
```
EXCLUDE if title matches:
- /^(Register|Sign up|Login|Subscribe)/i
- /^(Listings? By|Directory|Careers|Jobs)/i
- /Classifieds?$/i
- /^\w+ (Physio|Clinic|Centre|Center|Service)$/i  # Business listings
- /^Local (Sports|Events|Weather)$/i  # Section pages
- /^Spotlight$/i
```

### Non-Crime Context Exclusions
```
EXCLUDE if title matches AND body lacks crime keywords:
- /retirement|retires|retiring/i  # Personnel news
- /budget|funding|grants?/i  # Administrative
- /election|campaign|vote|ballot/i  # Politics without crime
- /recipe|restaurant|dining/i  # Food
- /weather|forecast|storm warning/i  # Weather
- /concert|festival|exhibition/i  # Entertainment
```

### Job Listing Detection
```
EXCLUDE if title contains job indicators:
- /(Part.Time|Full.Time|Hiring|Position|Vacancy)/i
- /Registered Nurse|RN|PSW|Technician/i + /Centre|Hospital|Clinic/i
```

---

## 2. Core Street Crime Patterns (High Precision)

### Violent Crime Incidents
```
CORE_STREET_CRIME + VIOLENT_CRIME if:
  title matches: /(murder|homicide|manslaughter)/i
  OR title matches: /(shoot|shot|shooting|gunfire)/i + NOT /(photo shoot|film shoot)/i
  OR title matches: /(stab|stabbing|stabbed|knife attack)/i
  OR title matches: /(assault|assaulted|beaten|beating)/i + /(charged|arrest|police|victim)/i
  OR title matches: /(kidnap|abduct|hostage)/i
  OR title matches: /(sexual assault|rape|sex assault)/i
  OR title matches: /human remains found/i
  OR title matches: /(armed robbery|robbery.+weapon|violent robbery)/i
```

### Property Crime Incidents
```
CORE_STREET_CRIME + PROPERTY_CRIME if:
  title matches: /(theft|stolen|shoplifting|larceny)/i + /(police|arrest|charged|suspect)/i
  OR title matches: /(burglary|break.in|breaking.+entering)/i
  OR title matches: /(arson)/i + NOT /(intent to harm|kill|murder)/i
  OR title matches: /(vandalism|vandalized|graffiti)/i + /(charged|arrest)/i
  OR title matches: /(car theft|auto theft|vehicle stolen|carjacking)/i
```

### Drug Crime Incidents
```
CORE_STREET_CRIME + DRUG_CRIME if:
  title matches: /(drug bust|drug raid|drug seizure)/i
  OR title matches: /(fentanyl|cocaine|heroin|meth|methamphetamine)/i + /(seiz|bust|arrest|charged|trafficking)/i
  OR title matches: /(trafficking|trafficker|dealer)/i + /(drug|narcotic|controlled substance)/i
  OR title matches: /possession.+(intent|purpose)/i
```

### Gang Violence
```
CORE_STREET_CRIME + GANG_VIOLENCE if:
  title matches: /(gang.related|gang member|gang shooting|gang violence)/i
  OR title matches: /(drive.by|turf war|gang rivalry)/i
  OR body contains: /gang/ + title matches violent_crime pattern
```

### Organized Crime
```
CORE_STREET_CRIME + ORGANIZED_CRIME if:
  title matches: /(organized crime|crime syndicate|crime ring)/i
  OR title matches: /(mafia|mob boss|cartel)/i
  OR title matches: /(human trafficking|trafficking ring)/i
  OR title matches: /(money laundering|RICO)/i
```

### Criminal Justice (Core)
```
CORE_STREET_CRIME + CRIMINAL_JUSTICE if:
  title matches: /(sentenced|sentencing|conviction|convicted)/i + (violent|property|drug crime keyword)
  OR title matches: /(trial|verdict|arraignment)/i + (murder|assault|robbery|trafficking)
  OR title matches: /(plead|pleads|pleaded) guilty/i + crime keyword
```

---

## 3. Peripheral Crime Patterns

### Impaired Driving (Peripheral by Default)
```
PERIPHERAL_CRIME + CRIMINAL_JUSTICE if:
  title matches: /(impaired driving|drunk driving|DUI|DWI)/i
  OR title matches: /(impaired|intoxicated).+(driver|driving|motorist)/i

UPGRADE to CORE_STREET_CRIME if:
  + /(collision|crash|hit.and.run|injur|fatal|kill)/i
```

### International Crime
```
PERIPHERAL_CRIME if:
  title contains international location indicators:
    /(Minneapolis|US|U\.S\.|American|Mexico|Dutch|European|Yemen|Myanmar|Congo|Venezuela)/i
  AND matches crime keywords

NEVER core_street_crime for international stories
```

### Policy/Statistics
```
PERIPHERAL_CRIME if:
  title matches: /(crime rate|crime statistics|crime down|crime up)/i
  OR title matches: /(police budget|policing policy|crime policy)/i
  OR title matches: /(crackdown on|spike in|rise in|wave of)/i + crime type
```

### Warnings Without Incidents
```
PERIPHERAL_CRIME if:
  title matches: /(police warn|warning|alert)/i + crime keyword
  AND NOT /(suspect|armed|dangerous|at large|flee)/i
```

### Victim Stories
```
PERIPHERAL_CRIME if:
  title matches: /(remembers|memorial|vigil|family of)/i + victim/crime keyword
  AND NOT /(new|update|investigation|arrest)/i
```

---

## 4. Location Specificity Rules

```
LOCAL_CANADA if title/body contains:
  - Ontario city: /(Sudbury|Sault|North Bay|Toronto|Ottawa|Hamilton|Barrie|Oshawa|Thunder Bay)/i
  - Quebec city: /(Montreal|Quebec City|Gatineau)/i
  - Other provinces: /(Vancouver|Calgary|Edmonton|Winnipeg|Halifax|Regina|Saskatoon)/i
  - Generic local: /(local police|area police|city police|regional police)/i

NATIONAL_CANADA if:
  - /(RCMP|federal|across Canada|nationwide|Canada-wide)/i
  - Multiple provinces mentioned

INTERNATIONAL if:
  - Non-Canadian location explicitly stated
  - /(US|American|Mexico|European|international)/i + crime context

NOT_SPECIFIED if:
  - No location indicators found
```

---

## 5. Suppression Rules (Prevent Over-Tagging)

### Remove Non-Crime Topics from Crime Articles
```
IF street_crime_relevance == core_street_crime:
  REMOVE: real_estate, home_garden, automotive, lifestyle, health,
          education, entertainment, sports, gaming, pets, shopping,
          food, travel, recreation, weather, science, technology,
          finance, business (unless white-collar crime)

  KEEP ONLY: violent_crime, property_crime, drug_crime, gang_violence,
             organized_crime, criminal_justice, other_crime
```

### Automotive Exception
```
KEEP automotive ONLY if:
  - Article is specifically about vehicle theft
  - Article is about impaired driving (peripheral)
```

### Real Estate Exception
```
NEVER tag real_estate on crime articles
The word "home" in "home invasion" is NOT real estate
The word "property" in "property crime" is NOT real estate
```

---

## 6. Confidence Scoring

### High Confidence (≥0.8)
- Multiple crime keywords in title
- Specific incident language (charged, arrested, shot, stabbed)
- Named victim or suspect
- Specific location

### Medium Confidence (0.5-0.79)
- Single crime keyword in title
- Generic police language
- No specific incident details

### Low Confidence (<0.5)
- Keywords only in body, not title
- Ambiguous context
- Mixed signals (crime + non-crime keywords)

---

## 7. Example Classifications

| Title | Relevance | Crime Types | Confidence |
|-------|-----------|-------------|------------|
| "Man charged with murder after downtown stabbing" | core | violent_crime, criminal_justice | 0.95 |
| "Police arrest two, seize $31K in drugs" | core | drug_crime, criminal_justice | 0.90 |
| "$3,500 in booze stolen from liquor store" | core | property_crime, criminal_justice | 0.85 |
| "Teen charged after hate-motivated vandalism" | core | property_crime, criminal_justice | 0.80 |
| "Human remains found east of North Bay" | core | violent_crime | 0.75 |
| "Sudbury police arrest two for impaired driving" | peripheral | criminal_justice | 0.70 |
| "US strikes drug boat after Maduro capture" | peripheral | drug_crime | 0.60 |
| "Mayor calls for crackdown on property crime" | peripheral | — | 0.50 |
| "Chief Justice to retire in May" | not_crime | — | — |
| "Registered Nurse position - Sexual Assault Centre" | not_crime | — | — |

---

## Version History

| Date | Change |
|------|--------|
| 2026-02-03 | Initial rule set |
