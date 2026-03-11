<?php

declare(strict_types=1);

namespace Database\Seeders;

use Illuminate\Database\Seeder;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\File;

/**
 * Seeds GIS community data from the canonical GeoJSON dataset.
 *
 * Loads communities.geojson (centroids + metadata) and individual boundary
 * polygons from data/gis/boundaries/ into the database.
 *
 * Usage: php artisan db:seed --class=GisCommunitySeeder
 */
class GisCommunitySeeder extends Seeder
{
    private const GIS_BASE_PATH = 'data/gis';
    private const COMMUNITIES_FILE = 'communities.geojson';
    private const BOUNDARIES_DIR = 'boundaries';

    public function run(): void
    {
        $basePath = base_path(self::GIS_BASE_PATH);
        $communitiesPath = $basePath . '/' . self::COMMUNITIES_FILE;

        if (! File::exists($communitiesPath)) {
            $this->command->error("Communities file not found: {$communitiesPath}");
            return;
        }

        $geojson = json_decode(File::get($communitiesPath), true);

        if (! $geojson || $geojson['type'] !== 'FeatureCollection') {
            $this->command->error('Invalid GeoJSON: expected FeatureCollection');
            return;
        }

        $this->command->info('Seeding ' . count($geojson['features']) . ' communities...');

        DB::transaction(function () use ($geojson, $basePath) {
            foreach ($geojson['features'] as $feature) {
                $this->seedCommunity($feature, $basePath);
            }
        });

        $this->command->info('GIS community seeding complete.');
    }

    private function seedCommunity(array $feature, string $basePath): void
    {
        $props = $feature['properties'];
        $coords = $feature['geometry']['coordinates'];
        $slug = $props['id'];

        $boundary = null;
        $boundaryFile = $props['boundary_file'] ?? null;

        if ($boundaryFile) {
            $boundaryPath = $basePath . '/' . $boundaryFile;
            if (File::exists($boundaryPath)) {
                $boundaryGeojson = json_decode(File::get($boundaryPath), true);
                if ($boundaryGeojson && isset($boundaryGeojson['features'][0]['geometry'])) {
                    $boundary = json_encode($boundaryGeojson['features'][0]['geometry']);
                }
            }
        }

        $externalIds = $props['external_ids'] ?? [];

        DB::table('gis_communities')->updateOrInsert(
            ['slug' => $slug],
            [
                'name' => $props['name'],
                'slug' => $slug,
                'type' => $props['type'],
                'municipality_type' => $props['municipality_type'] ?? null,
                'region' => $props['region'],
                'subregion' => $props['subregion'],
                'latitude' => $coords[1],
                'longitude' => $coords[0],
                'centroid' => DB::raw("ST_SetSRID(ST_MakePoint({$coords[0]}, {$coords[1]}), 4326)"),
                'boundary' => $boundary
                    ? DB::raw("ST_SetSRID(ST_GeomFromGeoJSON('{$boundary}'), 4326)")
                    : null,
                'population' => $props['population'] ?? null,
                'population_year' => $props['population_year'] ?? null,
                'reserve_name' => $props['reserve_name'] ?? null,
                'inac_band_number' => $externalIds['inac_band_number'] ?? null,
                'statscan_csd' => $externalIds['statscan_csd'] ?? null,
                'osm_relation_id' => $externalIds['osm_relation_id'] ?? null,
                'wikidata_qid' => $externalIds['wikidata_qid'] ?? null,
                'neighbours' => json_encode($props['neighbours'] ?? []),
                'created_at' => now(),
                'updated_at' => now(),
            ]
        );

        $this->command->line("  Seeded: {$props['name']}");
    }
}
