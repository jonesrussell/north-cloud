#!/usr/bin/env python3
"""Apply classification rules to exported dataset and generate analysis."""

import json
import re
from collections import Counter
from dataclasses import dataclass, field
from typing import Optional

@dataclass
class Classification:
    relevance: str  # core_street_crime, peripheral_crime, not_crime
    crime_types: list = field(default_factory=list)
    confidence: float = 0.0
    location: str = "not_specified"

# Exclusion patterns
EXCLUDE_TITLE_PATTERNS = [
    r'^(Register|Sign up|Login|Subscribe)',
    r'^(Listings? By|Directory|Careers|Jobs)',
    r'Classifieds?$',
    r'^\w+ (Physio|Clinic|Centre|Center|Service)$',
    r'^Local (Sports|Events|Weather)$',
    r'^Spotlight$',
    r'(Part.Time|Full.Time|Hiring|Position|Vacancy)',
]

NON_CRIME_TITLE_PATTERNS = [
    r'(retirement|retires|retiring)',
    r'(recipe|restaurant|dining|chef|cuisine)',
    r'(concert|festival|exhibition|lineup)',
    # Metaphorical uses of crime words
    r'shoots? down.*(rumour|bill|proposal|idea)',
    r'(water main|pipe) break',
    r'turning heads',  # sports idiom
    r'Trappers?.*\d{4}',  # sports team mentions
]

# Core street crime patterns
VIOLENT_CRIME_PATTERNS = [
    (r'(murder|homicide|manslaughter)', 0.95),
    # Shooting - exclude metaphorical uses like "shoots down", "photo shoot"
    (r'(?<!photo )(?<!film )(shooting|shootout|shot dead|shots fired|gunfire|gunshot)', 0.90),
    (r'(stab|stabbing|stabbed|knife attack)', 0.90),
    (r'(assault|assaulted|beaten|beating).*(charged|arrest|police|victim|hospital)', 0.85),
    # Assault with context - catches "charged with assault", "multiple assaults"
    (r'(charged|multiple|violent).*(assault|assaults)', 0.85),
    (r'assault.*(charged|arrest|melee|victim)', 0.85),
    (r'(kidnap|abduct|hostage)', 0.90),
    (r'(sexual assault|rape|sex assault|sexual exploitation)', 0.90),
    # Found dead - potential homicide
    (r'(human remains|body|person|child|man|woman) found dead', 0.80),
    (r'found dead.*(police|investigate|home|property)', 0.80),
    (r'(armed robbery|robbery.+weapon)', 0.85),
    (r'(hit.and.run|hit and run).*(dies|dead|fatal|kill|charged)', 0.85),
    # Attack patterns - security guard attacked, etc.
    (r'(attack|attacked).*(charged|arrest|victim|injur|police|guard)', 0.80),
    (r'(guard|officer|worker) attacked', 0.80),
    # Domestic/intimate partner violence
    (r'(intimate partner|domestic).*(violence|assault)', 0.85),
    (r'(fleeing|fled).*(violence|assault)', 0.80),
    # Child endangerment
    (r'(baby|child|infant).*(abandoned|abandonment)', 0.85),
    # Pleads guilty to violent crimes
    (r'pleads? guilty.*(assault|attack|violence)', 0.85),
    # Lacerations/injuries from incident
    (r'arrested.*(laceration|hospitalized|injur)', 0.80),
]

PROPERTY_CRIME_PATTERNS = [
    (r'(theft|stolen|shoplifting|larceny).*(police|arrest|charged|suspect|investigating)', 0.85),
    # Stolen without arrest - still crime if police involved
    (r'(police|investigating).*(stolen|theft)', 0.80),
    (r'(mezuzah|items?|property) stolen', 0.80),
    # Mass arrests for theft
    (r'\d+ arrests?.*(theft|retail|shoplifting)', 0.85),
    (r'arrests?.*(charges?|laid).*(theft|retail)', 0.85),
    # Exclude water/pipe breaks from burglary pattern
    (r'(burglary|break.in|breaking.+entering)(?!.*(water|pipe|main))', 0.85),
    (r'arson(?!.*(water|pipe))', 0.80),
    (r'(vandalism|vandalized).*(charged|arrest)', 0.80),
    (r'(car theft|auto theft|vehicle stolen|carjacking)', 0.85),
    (r'\$[\d,]+.*(stolen|theft|booze stolen)', 0.85),
    # Rammed/stolen vehicle
    (r'(rammed|ram).*(stolen|police)', 0.80),
    (r'stolen vehicle.*(ram|cruiser|police)', 0.80),
]

DRUG_CRIME_PATTERNS = [
    (r'(drug bust|drug raid|drug seizure)', 0.90),
    (r'(fentanyl|cocaine|heroin|meth|methamphetamine).*(seiz|bust|arrest|charged|trafficking)', 0.90),
    (r'(trafficking|trafficker|dealer).*(drug|narcotic)', 0.85),
    (r'possession.+(intent|purpose)', 0.80),
    (r'seize.+\$[\d,]+.*(drug|cocaine|fentanyl)', 0.90),
    # Seizes drugs pattern
    (r'seizes?.*(fentanyl|cocaine|heroin|drug|narcotic|doses)', 0.90),
    (r'(fentanyl|cocaine|heroin).*(doses|seized|weapons)', 0.90),
    # Intercept traffickers
    (r'intercept.*(fentanyl|cocaine|trafficker|drug)', 0.90),
    (r'(fentanyl|drug) trafficker', 0.90),
]

GANG_PATTERNS = [
    (r'(gang.related|gang member|gang shooting|gang violence)', 0.90),
    (r'(drive.by|turf war|gang rivalry)', 0.85),
]

ORGANIZED_CRIME_PATTERNS = [
    (r'(organized crime|crime syndicate|crime ring)', 0.85),
    (r'(mafia|mob boss|cartel)', 0.85),
    (r'(human trafficking|trafficking ring)', 0.85),
    (r'(money laundering|RICO)', 0.80),
]

CRIMINAL_JUSTICE_PATTERNS = [
    (r'(charged|arrest|arrested)', 0.70),
    (r'(sentenced|sentencing|conviction|convicted)', 0.75),
    (r'(trial|verdict|arraignment)', 0.70),
    (r'(plead|pleads|pleaded) guilty', 0.80),
    (r'warrant', 0.65),
    # Standoffs leading to arrest
    (r'standoff.*(arrest|charged|custody)', 0.80),
    (r'(police|armed) standoff', 0.75),
    # Prison/jail escapes
    (r'escapes?.*(jail|prison|custody)', 0.85),
    (r'(jail|prison) escape', 0.85),
]

# Peripheral patterns
IMPAIRED_DRIVING_PATTERNS = [
    r'(impaired driving|drunk driving|DUI|DWI)',
    r'(impaired|intoxicated).+(driver|driving|motorist)',
]

INTERNATIONAL_INDICATORS = [
    r'Minneapolis', r'\bUS\b', r'U\.S\.', r'American', r'Mexico', r'Dutch',
    r'European', r'Yemen', r'Myanmar', r'Congo', r'Venezuela', r'Israel',
    r'Gaza', r'Greenland', r'Danish', r'Thailand', r'Cambodia', r'Malaysia',
]

POLICY_PATTERNS = [
    r'(crime rate|crime statistics|crime down|crime up)',
    r'(police budget|policing policy|crime policy)',
    r'(crackdown on|spike in|rise in|wave of)',
    r'(mayor|council|government).*(crime|police)',
    r'(declares|declared|address|addressing).*(epidemic|crisis)',
    r'^In the news today:',  # News roundups
]

# Peripheral crime patterns - not core but still crime-related
PERIPHERAL_SPECIFIC_PATTERNS = [
    r'stole.*(returned|return)',  # Theft but returned
    r'awaits? Crown.*(decision|charges)',  # Pending charges
    r'(law licence|licence assessment).*(abuse|assault)',  # Historical crimes, professional matters
    r'(cleared|exonerated).*(arrest|force)',  # Police cleared after use of force
    r'Year in review.*(crime|high crimes)',  # Retrospective
]

# Canadian location indicators
CANADIAN_LOCATIONS = [
    r'Sudbury', r'Sault', r'North Bay', r'Toronto', r'Ottawa', r'Hamilton',
    r'Barrie', r'Oshawa', r'Thunder Bay', r'Montreal', r'Vancouver', r'Calgary',
    r'Edmonton', r'Winnipeg', r'Halifax', r'Regina', r'Saskatoon', r'Brampton',
    r'Mississauga', r'London', r'Ontario', r'Quebec', r'Manitoba', r'Alberta',
    r'British Columbia', r'Nova Scotia', r'Ont\.', r'Que\.',
]


def match_patterns(text: str, patterns: list) -> tuple[bool, float]:
    """Check if text matches any pattern, return (matched, max_confidence)."""
    text = text.lower()
    max_conf = 0.0
    for pattern, conf in patterns:
        if re.search(pattern, text, re.IGNORECASE):
            max_conf = max(max_conf, conf)
    return max_conf > 0, max_conf


def match_any(text: str, patterns: list) -> bool:
    """Check if text matches any pattern (no confidence)."""
    text = text.lower()
    for pattern in patterns:
        if re.search(pattern, text, re.IGNORECASE):
            return True
    return False


def classify_article(title: str, body: str = "") -> Classification:
    """Classify an article using the rule set."""
    title = title or ""
    body = body or ""
    combined = f"{title} {body[:500]}"  # Use title + first 500 chars of body

    result = Classification(relevance="not_crime")

    # Step 1: Check exclusions
    for pattern in EXCLUDE_TITLE_PATTERNS:
        if re.search(pattern, title, re.IGNORECASE):
            return result

    # Check non-crime patterns
    for pattern in NON_CRIME_TITLE_PATTERNS:
        if re.search(pattern, title, re.IGNORECASE):
            # Only exclude if no strong crime keywords present
            if not re.search(r'(murder|homicide|assault|theft|robbery|arrest|charged)', title, re.IGNORECASE):
                return result

    # Additional exclusions for common false positive patterns
    if re.search(r'(water main|pipe|infrastructure).*(break|burst)', title, re.IGNORECASE):
        return result
    if re.search(r'(Trappers?|Wolves|Greyhounds|Battalions?).*(hockey|\d{4})', title, re.IGNORECASE):
        return result
    if re.search(r'shoots? down.*(rumour|bill|proposal)', title, re.IGNORECASE):
        return result

    # Step 2: Check for core street crime patterns
    crime_types = []
    max_confidence = 0.0

    # Violent crime
    matched, conf = match_patterns(title, VIOLENT_CRIME_PATTERNS)
    if matched:
        crime_types.append("violent_crime")
        max_confidence = max(max_confidence, conf)

    # Property crime
    matched, conf = match_patterns(title, PROPERTY_CRIME_PATTERNS)
    if matched:
        crime_types.append("property_crime")
        max_confidence = max(max_confidence, conf)

    # Drug crime
    matched, conf = match_patterns(title, DRUG_CRIME_PATTERNS)
    if matched:
        crime_types.append("drug_crime")
        max_confidence = max(max_confidence, conf)

    # Gang violence
    matched, conf = match_patterns(title, GANG_PATTERNS)
    if matched:
        crime_types.append("gang_violence")
        max_confidence = max(max_confidence, conf)

    # Organized crime
    matched, conf = match_patterns(title, ORGANIZED_CRIME_PATTERNS)
    if matched:
        crime_types.append("organized_crime")
        max_confidence = max(max_confidence, conf)

    # Criminal justice (modifier)
    matched, conf = match_patterns(title, CRIMINAL_JUSTICE_PATTERNS)
    if matched:
        # Check for standalone criminal justice incidents (escapes, standoffs)
        if re.search(r'escapes?.*(jail|prison|custody)', title, re.IGNORECASE):
            crime_types.append("other_crime")
            max_confidence = max(max_confidence, conf)
        if re.search(r'standoff.*(arrest|charged)', title, re.IGNORECASE):
            max_confidence = max(max_confidence, conf)
        # Only add criminal_justice as modifier if other crime type present or standalone incident
        if crime_types or re.search(r'(standoff|escape)', title, re.IGNORECASE):
            crime_types.append("criminal_justice")

    # Step 3: Check for peripheral patterns
    is_international = match_any(title, INTERNATIONAL_INDICATORS)
    is_impaired = match_any(title, IMPAIRED_DRIVING_PATTERNS)
    is_policy = match_any(title, POLICY_PATTERNS)
    is_peripheral_specific = match_any(title, PERIPHERAL_SPECIFIC_PATTERNS)

    # Peripheral-specific patterns (theft returned, pending charges, historical)
    if is_peripheral_specific:
        result.relevance = "peripheral_crime"
        # Determine crime types for peripheral
        if re.search(r'stole|theft', title, re.IGNORECASE):
            result.crime_types = ["property_crime", "criminal_justice"]
        elif re.search(r'assault|abuse|violence', title, re.IGNORECASE):
            result.crime_types = ["violent_crime", "criminal_justice"]
        else:
            result.crime_types = ["criminal_justice"]
        result.confidence = 0.60
        return result

    # Policy/epidemic declarations override crime patterns
    if is_policy and not re.search(r'(charged|arrest|sentenced|murder|shot|stabbed)', title, re.IGNORECASE):
        result.relevance = "peripheral_crime"
        result.crime_types = crime_types if crime_types else []
        result.confidence = 0.50
        return result

    # Impaired driving - peripheral unless collision/injury
    if is_impaired:
        if re.search(r'(collision|crash|hit.and.run|injur|fatal|kill|dies)', title, re.IGNORECASE):
            crime_types.append("violent_crime")
            result.relevance = "core_street_crime"
            max_confidence = max(max_confidence, 0.80)
        else:
            result.relevance = "peripheral_crime"
            crime_types = ["criminal_justice"]
            max_confidence = 0.70
    # International crime - always peripheral
    elif is_international and crime_types:
        result.relevance = "peripheral_crime"
        max_confidence = min(max_confidence, 0.65)
    # Policy/statistics - peripheral
    elif is_policy:
        result.relevance = "peripheral_crime"
        max_confidence = 0.50
    # Core street crime
    elif crime_types:
        result.relevance = "core_street_crime"

    # Step 4: Determine location
    if match_any(combined, CANADIAN_LOCATIONS):
        result.location = "local_canada"
    elif match_any(combined, [r'RCMP', r'federal', r'across Canada', r'nationwide']):
        result.location = "national_canada"
    elif is_international:
        result.location = "international"

    result.crime_types = crime_types
    result.confidence = max_confidence

    return result


def main():
    # Load exported data
    articles = []
    with open('/tmp/streetcode_export.jsonl', 'r') as f:
        for line in f:
            articles.append(json.loads(line))

    print(f"Loaded {len(articles)} articles\n")

    # Classify all articles
    results = []
    for article in articles:
        classification = classify_article(article['title'], article.get('raw_text', ''))
        results.append({
            'id': article['id'],
            'title': article['title'],
            'current_topics': article.get('topics', []),
            'new_relevance': classification.relevance,
            'new_crime_types': classification.crime_types,
            'new_location': classification.location,
            'confidence': classification.confidence,
        })

    # Generate statistics
    print("=" * 60)
    print("NEW CLASSIFICATION DISTRIBUTION")
    print("=" * 60)

    relevance_counts = Counter(r['new_relevance'] for r in results)
    print("\nStreet Crime Relevance:")
    for rel, count in relevance_counts.most_common():
        pct = count / len(results) * 100
        print(f"  {rel}: {count} ({pct:.1f}%)")

    crime_type_counts = Counter()
    for r in results:
        for ct in r['new_crime_types']:
            crime_type_counts[ct] += 1

    print("\nCrime Types (multi-label):")
    for ct, count in crime_type_counts.most_common():
        print(f"  {ct}: {count}")

    location_counts = Counter(r['new_location'] for r in results)
    print("\nLocation Specificity:")
    for loc, count in location_counts.most_common():
        print(f"  {loc}: {count}")

    # Confusion analysis
    print("\n" + "=" * 60)
    print("CONFUSION ANALYSIS vs CURRENT LABELS")
    print("=" * 60)

    # Count current crime vs new crime
    current_has_crime = 0
    new_has_crime = 0
    both_crime = 0
    current_only = 0
    new_only = 0

    crime_topics = {'violent_crime', 'property_crime', 'drug_crime', 'gang_violence',
                    'organized_crime', 'criminal_justice', 'crime'}

    for r in results:
        current = set(r['current_topics'] or [])
        has_current_crime = bool(current & crime_topics)
        has_new_crime = r['new_relevance'] in ('core_street_crime', 'peripheral_crime')

        if has_current_crime:
            current_has_crime += 1
        if has_new_crime:
            new_has_crime += 1
        if has_current_crime and has_new_crime:
            both_crime += 1
        if has_current_crime and not has_new_crime:
            current_only += 1
        if has_new_crime and not has_current_crime:
            new_only += 1

    print(f"\nCurrent system flagged as crime: {current_has_crime}")
    print(f"New rules flagged as crime: {new_has_crime}")
    print(f"Agreement (both crime): {both_crime}")
    print(f"Current-only (potential false positives): {current_only}")
    print(f"New-only (potential false negatives fixed): {new_only}")

    # Show examples
    print("\n" + "=" * 60)
    print("SAMPLE CORRECTIONS")
    print("=" * 60)

    print("\n--- FALSE NEGATIVES FIXED (crime articles now properly tagged) ---")
    fn_fixed = [r for r in results if r['new_relevance'] == 'core_street_crime'
                and not (set(r['current_topics'] or []) & crime_topics)]
    for r in fn_fixed[:10]:
        print(f"\n  Title: {r['title'][:80]}")
        print(f"  Old: {r['current_topics']}")
        print(f"  New: {r['new_relevance']} / {r['new_crime_types']}")

    print("\n--- FALSE POSITIVES FIXED (non-crime now correctly excluded) ---")
    fp_fixed = [r for r in results if r['new_relevance'] == 'not_crime'
                and (set(r['current_topics'] or []) & crime_topics)]
    for r in fp_fixed[:10]:
        print(f"\n  Title: {r['title'][:80]}")
        print(f"  Old: {r['current_topics']}")
        print(f"  New: {r['new_relevance']}")

    print("\n--- OVER-TAGGING FIXED (crime articles with cleaner labels) ---")
    overtagged_fixed = [r for r in results if r['new_relevance'] == 'core_street_crime'
                        and len(r['current_topics'] or []) > 4]
    for r in overtagged_fixed[:10]:
        print(f"\n  Title: {r['title'][:80]}")
        print(f"  Old: {r['current_topics']}")
        print(f"  New: {r['new_crime_types']}")

    # Save results
    with open('/tmp/streetcode_classified.jsonl', 'w') as f:
        for r in results:
            f.write(json.dumps(r) + '\n')

    print(f"\n\nResults saved to /tmp/streetcode_classified.jsonl")

    # Generate CSV for manual review
    print("\n" + "=" * 60)
    print("CORE STREET CRIME ARTICLES FOR HOMEPAGE")
    print("=" * 60)

    core_articles = [r for r in results if r['new_relevance'] == 'core_street_crime']
    print(f"\nTotal core street crime: {len(core_articles)}")
    print("\nSample headlines for homepage:")
    for r in sorted(core_articles, key=lambda x: -x['confidence'])[:20]:
        print(f"  [{r['confidence']:.2f}] {r['title'][:70]}")


if __name__ == '__main__':
    main()

# Export CSV for manual labeling
import csv

print("\n" + "=" * 60)
print("EXPORTING CSV FOR MANUAL LABELING")
print("=" * 60)

# Select 300 diverse articles for manual review
core_articles = [r for r in results if r['new_relevance'] == 'core_street_crime']
peripheral_articles = [r for r in results if r['new_relevance'] == 'peripheral_crime']
not_crime_with_keywords = [r for r in results if r['new_relevance'] == 'not_crime'
                          and re.search(r'(police|arrest|crime|court|theft|drug)', r['title'], re.IGNORECASE)]
random_not_crime = [r for r in results if r['new_relevance'] == 'not_crime'
                   and not re.search(r'(police|arrest|crime|court|theft|drug)', r['title'], re.IGNORECASE)]

import random
random.seed(42)
random.shuffle(random_not_crime)

manual_review = (
    core_articles[:60] +
    peripheral_articles[:25] +
    not_crime_with_keywords[:100] +
    random_not_crime[:115]
)

print(f"Selected {len(manual_review)} articles for manual review:")
print(f"  - Core street crime: {len([r for r in manual_review if r['new_relevance'] == 'core_street_crime'])}")
print(f"  - Peripheral crime: {len([r for r in manual_review if r['new_relevance'] == 'peripheral_crime'])}")
print(f"  - Not crime (keyword): {len([r for r in manual_review if r['new_relevance'] == 'not_crime' and re.search(r'(police|arrest|crime|court|theft|drug)', r['title'], re.IGNORECASE)])}")
print(f"  - Not crime (random): {len([r for r in manual_review if r['new_relevance'] == 'not_crime' and not re.search(r'(police|arrest|crime|court|theft|drug)', r['title'], re.IGNORECASE)])}")

with open('/tmp/streetcode_manual_review.csv', 'w', newline='') as f:
    writer = csv.writer(f)
    writer.writerow(['id', 'title', 'current_topics', 'rule_relevance', 'rule_crime_types', 'rule_confidence',
                     'manual_relevance', 'manual_crime_types', 'notes'])
    for r in manual_review:
        writer.writerow([
            r['id'],
            r['title'][:100],
            '|'.join(r['current_topics'] or []),
            r['new_relevance'],
            '|'.join(r['new_crime_types']),
            f"{r['confidence']:.2f}",
            '',  # manual_relevance - to be filled
            '',  # manual_crime_types - to be filled
            '',  # notes - to be filled
        ])

print(f"\nCSV exported to /tmp/streetcode_manual_review.csv")
