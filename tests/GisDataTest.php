<?php

declare(strict_types=1);

namespace Tests;

use PHPUnit\Framework\TestCase;

/**
 * Validates GIS data integrity for the North Shore community dataset.
 *
 * Usage: cd data/gis && php ../../vendor/bin/phpunit ../../tests/GisDataTest.php
 *    or: php -r "require 'tests/GisDataTest.php'; (new Tests\GisDataTest())->runAll();"
 */
class GisDataTest extends TestCase
{
    private const GIS_BASE = __DIR__ . '/../data/gis';
    private const COMMUNITIES_FILE = self::GIS_BASE . '/communities.geojson';
    private const BOUNDARIES_DIR = self::GIS_BASE . '/boundaries';

    private array $geojson;
    private array $features;

    protected function setUp(): void
    {
        parent::setUp();

        $this->assertFileExists(self::COMMUNITIES_FILE, 'communities.geojson must exist');

        $raw = file_get_contents(self::COMMUNITIES_FILE);
        $this->assertNotFalse($raw);

        $this->geojson = json_decode($raw, true);
        $this->assertNotNull($this->geojson, 'communities.geojson must be valid JSON');

        $this->features = $this->geojson['features'] ?? [];
    }

    public function testIsValidFeatureCollection(): void
    {
        $this->assertSame('FeatureCollection', $this->geojson['type']);
        $this->assertNotEmpty($this->features, 'Must have at least one feature');
    }

    public function testAllFeaturesHaveRequiredProperties(): void
    {
        $requiredProps = ['id', 'name', 'type', 'region', 'external_ids'];

        foreach ($this->features as $i => $feature) {
            $this->assertSame('Feature', $feature['type'], "Feature {$i} must be type Feature");
            $this->assertArrayHasKey('properties', $feature, "Feature {$i} missing properties");
            $this->assertArrayHasKey('geometry', $feature, "Feature {$i} missing geometry");

            foreach ($requiredProps as $prop) {
                $this->assertArrayHasKey(
                    $prop,
                    $feature['properties'],
                    "Feature {$i} ({$feature['properties']['name'] ?? 'unknown'}) missing property: {$prop}"
                );
            }
        }
    }

    public function testCoordinatesAreValid(): void
    {
        foreach ($this->features as $feature) {
            $name = $feature['properties']['name'];
            $geom = $feature['geometry'];

            $this->assertSame('Point', $geom['type'], "{$name}: centroid must be Point");
            $this->assertCount(2, $geom['coordinates'], "{$name}: coordinates must be [lon, lat]");

            [$lon, $lat] = $geom['coordinates'];

            // North Shore of Lake Huron bounding box
            $this->assertGreaterThanOrEqual(-85.0, $lon, "{$name}: longitude out of range");
            $this->assertLessThanOrEqual(-79.0, $lon, "{$name}: longitude out of range");
            $this->assertGreaterThanOrEqual(45.5, $lat, "{$name}: latitude out of range");
            $this->assertLessThanOrEqual(47.5, $lat, "{$name}: latitude out of range");
        }
    }

    public function testNoDuplicateIds(): void
    {
        $ids = array_map(fn($f) => $f['properties']['id'], $this->features);
        $duplicates = array_diff_assoc($ids, array_unique($ids));

        $this->assertEmpty($duplicates, 'Duplicate IDs found: ' . implode(', ', $duplicates));
    }

    public function testAllFirstNationsHaveInacIds(): void
    {
        foreach ($this->features as $feature) {
            $props = $feature['properties'];

            if ($props['type'] !== 'first_nation') {
                continue;
            }

            $this->assertArrayHasKey(
                'inac_band_number',
                $props['external_ids'],
                "{$props['name']}: First Nation must have INAC band number"
            );

            $this->assertNotNull(
                $props['external_ids']['inac_band_number'],
                "{$props['name']}: INAC band number must not be null"
            );
        }
    }

    public function testAllCommunitiesHaveStatsCan(): void
    {
        foreach ($this->features as $feature) {
            $props = $feature['properties'];

            $this->assertArrayHasKey(
                'statscan_csd',
                $props['external_ids'],
                "{$props['name']}: must have StatsCan CSD code"
            );

            $this->assertNotNull(
                $props['external_ids']['statscan_csd'],
                "{$props['name']}: StatsCan CSD must not be null"
            );

            // CSD codes are 7 digits for Ontario
            $this->assertMatchesRegularExpression(
                '/^35\d{5}$/',
                $props['external_ids']['statscan_csd'],
                "{$props['name']}: StatsCan CSD must be 7-digit Ontario code (35xxxxx)"
            );
        }
    }

    public function testCommunityTypesAreValid(): void
    {
        $validTypes = ['first_nation', 'municipality'];

        foreach ($this->features as $feature) {
            $this->assertContains(
                $feature['properties']['type'],
                $validTypes,
                "{$feature['properties']['name']}: invalid type '{$feature['properties']['type']}'"
            );
        }
    }

    public function testBoundaryFilesExist(): void
    {
        foreach ($this->features as $feature) {
            $props = $feature['properties'];
            $boundaryFile = $props['boundary_file'] ?? null;

            if ($boundaryFile === null) {
                // Boundary is expected to be missing (e.g., Wahnapitae FN)
                $this->assertTrue(
                    $props['boundary_missing'] ?? false,
                    "{$props['name']}: missing boundary_file without boundary_missing flag"
                );
                continue;
            }

            $path = self::GIS_BASE . '/' . $boundaryFile;
            $this->assertFileExists($path, "{$props['name']}: boundary file not found: {$boundaryFile}");

            $boundaryJson = json_decode(file_get_contents($path), true);
            $this->assertNotNull($boundaryJson, "{$props['name']}: boundary file is not valid JSON");
            $this->assertSame(
                'FeatureCollection',
                $boundaryJson['type'],
                "{$props['name']}: boundary must be FeatureCollection"
            );
        }
    }

    public function testBoundaryPolygonsAreValid(): void
    {
        $boundaryFiles = glob(self::BOUNDARIES_DIR . '/*.geojson');
        $this->assertNotEmpty($boundaryFiles, 'No boundary files found');

        foreach ($boundaryFiles as $path) {
            $filename = basename($path);
            $json = json_decode(file_get_contents($path), true);

            $this->assertNotNull($json, "{$filename}: invalid JSON");
            $this->assertSame('FeatureCollection', $json['type'], "{$filename}: must be FeatureCollection");
            $this->assertNotEmpty($json['features'], "{$filename}: must have features");

            $geom = $json['features'][0]['geometry'];
            $this->assertContains(
                $geom['type'],
                ['Polygon', 'MultiPolygon'],
                "{$filename}: geometry must be Polygon or MultiPolygon"
            );

            // Validate polygon rings are closed
            $this->assertPolygonRingsClosed($geom, $filename);
        }
    }

    public function testNeighboursAreBidirectional(): void
    {
        $communityMap = [];
        foreach ($this->features as $feature) {
            $communityMap[$feature['properties']['id']] = $feature['properties']['neighbours'] ?? [];
        }

        foreach ($communityMap as $id => $neighbours) {
            foreach ($neighbours as $neighbourId) {
                $this->assertArrayHasKey(
                    $neighbourId,
                    $communityMap,
                    "{$id}: neighbour '{$neighbourId}' does not exist in communities"
                );

                $this->assertContains(
                    $id,
                    $communityMap[$neighbourId],
                    "{$id} lists {$neighbourId} as neighbour, but {$neighbourId} does not list {$id}"
                );
            }
        }
    }

    public function testPopulationDataPresent(): void
    {
        foreach ($this->features as $feature) {
            $props = $feature['properties'];
            $this->assertArrayHasKey('population', $props, "{$props['name']}: missing population");
            $this->assertIsInt($props['population'], "{$props['name']}: population must be integer");
            $this->assertGreaterThan(0, $props['population'], "{$props['name']}: population must be positive");
        }
    }

    private function assertPolygonRingsClosed(array $geometry, string $context): void
    {
        $polygons = $geometry['type'] === 'MultiPolygon'
            ? $geometry['coordinates']
            : [$geometry['coordinates']];

        foreach ($polygons as $pi => $polygon) {
            foreach ($polygon as $ri => $ring) {
                $first = $ring[0];
                $last = $ring[count($ring) - 1];
                $this->assertSame(
                    $first,
                    $last,
                    "{$context}: polygon {$pi} ring {$ri} is not closed"
                );
            }
        }
    }

    /**
     * Run all tests without PHPUnit framework (standalone execution).
     */
    public function runAll(): void
    {
        $this->setUp();

        $methods = array_filter(
            get_class_methods($this),
            fn($m) => str_starts_with($m, 'test')
        );

        $passed = 0;
        $failed = 0;

        foreach ($methods as $method) {
            try {
                $this->$method();
                echo "  PASS: {$method}\n";
                $passed++;
            } catch (\Throwable $e) {
                echo "  FAIL: {$method}\n";
                echo "        {$e->getMessage()}\n";
                $failed++;
            }
        }

        echo "\nResults: {$passed} passed, {$failed} failed\n";

        if ($failed > 0) {
            exit(1);
        }
    }
}

// Allow standalone execution
if (php_sapi_name() === 'cli' && realpath($argv[0] ?? '') === __FILE__) {
    (new GisDataTest())->runAll();
}
