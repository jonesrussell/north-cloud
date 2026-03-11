# GIS Data - North Shore of Lake Huron

Canonical geospatial dataset for all communities along the North Shore of Lake Huron,
from Sault Ste. Marie to Greater Sudbury. Used by Minoo, Waaseyaa, and NorthCloud.

## Data Sources

| Source | URL | Data Type |
|--------|-----|-----------|
| OpenStreetMap (Nominatim) | nominatim.openstreetmap.org | Centroids, boundary polygons |
| OpenStreetMap (Overpass) | overpass-api.de | Boundary relations |
| Wikidata | wikidata.org | QIDs, cross-references |
| Statistics Canada (2021 Census) | statcan.gc.ca | CSD codes, population |
| Indigenous Services Canada | sac-isc.gc.ca | INAC band numbers, reserve names |

## File Structure

```
data/gis/
  communities.geojson       # FeatureCollection of all communities (Point centroids)
  boundaries/
    <community-slug>.geojson # Individual boundary polygons (MultiPolygon)
  sources/
    *.geojson                # Raw OSM polygon downloads
```

## Coordinate System

All coordinates use **WGS84 (EPSG:4326)** / CRS84 (longitude, latitude order per GeoJSON spec).

- Longitude is the first coordinate (negative for Western Hemisphere)
- Latitude is the second coordinate
- All values are in decimal degrees

## Community Types

| Type | Count | Description |
|------|-------|-------------|
| `first_nation` | 9 | First Nation reserves / Indian Reserves |
| `municipality` | 11 | Towns, cities, townships |

## External ID Reference

| ID Type | Property Path | Example | Source |
|---------|---------------|---------|--------|
| INAC Band Number | `external_ids.inac_band_number` | 179 | ISC First Nation Profiles |
| StatsCan CSD | `external_ids.statscan_csd` | "3557071" | Statistics Canada |
| OSM Relation ID | `external_ids.osm_relation_id` | 7589023 | OpenStreetMap |
| Wikidata QID | `external_ids.wikidata_qid` | "Q7398967" | Wikidata |

## Update Workflow

1. **Adding a new community**:
   - Add centroid Point feature to `communities.geojson`
   - Download boundary polygon via `https://polygons.openstreetmap.fr/get_geojson.py?id=RELATION_ID&params=0`
   - Save raw download to `sources/`
   - Wrap in FeatureCollection and save to `boundaries/<slug>.geojson`
   - Update neighbour lists for adjacent communities
   - Run validation tests: `php tests/GisDataTest.php`

2. **Updating population data**:
   - Update `population` and `population_year` in `communities.geojson`
   - Source: Statistics Canada Census (every 5 years)

3. **Updating boundaries**:
   - Re-download from OSM polygon service
   - Replace raw file in `sources/`
   - Regenerate boundary FeatureCollection in `boundaries/`

## Normalization Rules

- Community slugs use lowercase kebab-case: `sagamok-anishnawbek`
- First Nation names use the band's preferred spelling (e.g., "Wahnapitae" not "Wahnapitei")
- Reserve names use official ISC/StatsCan names (e.g., "Sagamok 12")
- Population figures are on-reserve counts from the most recent Census
- Neighbour relationships are bidirectional (if A lists B, B must list A)

## Polygon Validation Rules

- All geometries must be valid GeoJSON (RFC 7946)
- Polygon rings must be closed (first point == last point)
- Exterior rings must be counter-clockwise (right-hand rule)
- No self-intersections
- Coordinate precision: 7 decimal places maximum
- All boundaries must be Polygon or MultiPolygon type

## Known Data Gaps

- **Wahnapitae First Nation**: No OSM boundary polygon (reserve not mapped in OSM)
- **Iron Bridge**: Unincorporated community within Huron Shores (no separate boundary)
- **McKerrow**: Unincorporated community within Baldwin Township (no separate boundary)
- **Nairn Centre**: Unincorporated community within Nairn and Hyman (no separate boundary)
- **Massey**: Unincorporated community within Sables-Spanish Rivers (no separate boundary)
