<?php

namespace Tests\Data;

use PHPUnit\Framework\TestCase;

class CommunityDataTest extends TestCase
{
    private array $communities;
    private string $jsonPath;

    private const REQUIRED_FIELDS = [
        'id', 'name', 'type', 'province', 'latitude', 'longitude',
        'governing_body', 'external_ids', 'region',
    ];

    private const VALID_TYPES = [
        'first_nation', 'reserve', 'town', 'city', 'municipality', 'settlement',
    ];

    protected function setUp(): void
    {
        $this->jsonPath = __DIR__ . '/../communities.json';
        $this->assertFileExists($this->jsonPath, 'communities.json must exist');

        $json = file_get_contents($this->jsonPath);
        $this->communities = json_decode($json, true);
        $this->assertNotNull($this->communities, 'communities.json must be valid JSON');
        $this->assertIsArray($this->communities);
        $this->assertNotEmpty($this->communities, 'communities.json must not be empty');
    }

    public function testJsonStructureIsValid(): void
    {
        foreach ($this->communities as $index => $community) {
            foreach (self::REQUIRED_FIELDS as $field) {
                $this->assertArrayHasKey(
                    $field,
                    $community,
                    sprintf('Community at index %d (%s) is missing required field: %s', $index, $community['name'] ?? 'unknown', $field)
                );
            }
        }
    }

    public function testTypesAreValid(): void
    {
        foreach ($this->communities as $community) {
            $this->assertContains(
                $community['type'],
                self::VALID_TYPES,
                "Community '{$community['name']}' has invalid type: {$community['type']}"
            );
        }
    }

    public function testProvinceIsOntario(): void
    {
        foreach ($this->communities as $community) {
            $this->assertEquals(
                'ON',
                $community['province'],
                "Community '{$community['name']}' must have province 'ON' for North Shore dataset"
            );
        }
    }

    public function testCoordinatesAreValid(): void
    {
        foreach ($this->communities as $community) {
            $lat = $community['latitude'];
            $lon = $community['longitude'];

            $this->assertIsFloat($lat, "Latitude for '{$community['name']}' must be a float");
            $this->assertIsFloat($lon, "Longitude for '{$community['name']}' must be a float");

            // North Shore of Lake Huron bounds (approximate)
            $this->assertGreaterThanOrEqual(45.5, $lat, "Latitude for '{$community['name']}' is too far south");
            $this->assertLessThanOrEqual(47.5, $lat, "Latitude for '{$community['name']}' is too far north");
            $this->assertGreaterThanOrEqual(-85.0, $lon, "Longitude for '{$community['name']}' is too far west");
            $this->assertLessThanOrEqual(-80.0, $lon, "Longitude for '{$community['name']}' is too far east");
        }
    }

    public function testNoDuplicateNames(): void
    {
        $names = array_column($this->communities, 'name');
        $uniqueNames = array_unique($names);

        $this->assertCount(
            count($names),
            $uniqueNames,
            'Duplicate community names found: ' . implode(', ', array_diff_assoc($names, $uniqueNames))
        );
    }

    public function testNoDuplicateIds(): void
    {
        $ids = array_column($this->communities, 'id');
        $uniqueIds = array_unique($ids);

        $this->assertCount(
            count($ids),
            $uniqueIds,
            'Duplicate community IDs found: ' . implode(', ', array_diff_assoc($ids, $uniqueIds))
        );
    }

    public function testAllFirstNationsHaveInacIds(): void
    {
        $firstNations = array_filter($this->communities, fn($c) => $c['type'] === 'first_nation');

        foreach ($firstNations as $fn) {
            $this->assertArrayHasKey(
                'external_ids',
                $fn,
                "First Nation '{$fn['name']}' must have external_ids"
            );
            $this->assertArrayHasKey(
                'inac',
                $fn['external_ids'],
                "First Nation '{$fn['name']}' must have an INAC ID in external_ids"
            );
            $this->assertNotEmpty(
                $fn['external_ids']['inac'],
                "First Nation '{$fn['name']}' INAC ID must not be empty"
            );
        }
    }

    public function testAllCommunitiesHaveStatcanCodes(): void
    {
        foreach ($this->communities as $community) {
            $this->assertArrayHasKey(
                'statcan',
                $community['external_ids'],
                "Community '{$community['name']}' must have a StatsCan code"
            );
            $this->assertMatchesRegularExpression(
                '/^\d{7}$/',
                $community['external_ids']['statcan'],
                "StatsCan code for '{$community['name']}' must be exactly 7 digits"
            );
        }
    }

    public function testRegionIsConsistent(): void
    {
        foreach ($this->communities as $community) {
            $this->assertEquals(
                'North Shore of Lake Huron',
                $community['region'],
                "Community '{$community['name']}' must have region 'North Shore of Lake Huron'"
            );
        }
    }

    public function testIdsAreSlugFormat(): void
    {
        foreach ($this->communities as $community) {
            $this->assertMatchesRegularExpression(
                '/^[a-z0-9]+(-[a-z0-9]+)*$/',
                $community['id'],
                "ID for '{$community['name']}' must be lowercase slug format"
            );
        }
    }

    public function testNeighboursReferenceValidIds(): void
    {
        $allIds = array_column($this->communities, 'id');

        foreach ($this->communities as $community) {
            if (!isset($community['neighbours'])) {
                continue;
            }

            foreach ($community['neighbours'] as $neighbour) {
                $this->assertContains(
                    $neighbour,
                    $allIds,
                    "Community '{$community['name']}' references unknown neighbour: {$neighbour}"
                );
            }
        }
    }

    public function testNdjsonFileMatchesJson(): void
    {
        $ndjsonPath = __DIR__ . '/../communities.ndjson';
        $this->assertFileExists($ndjsonPath, 'communities.ndjson must exist');

        $lines = array_filter(explode("\n", file_get_contents($ndjsonPath)), fn($l) => trim($l) !== '');

        $this->assertCount(
            count($this->communities),
            $lines,
            'communities.ndjson must have the same number of entries as communities.json'
        );

        foreach ($lines as $index => $line) {
            $decoded = json_decode($line, true);
            $this->assertNotNull($decoded, "NDJSON line {$index} is not valid JSON");
            $this->assertEquals(
                $this->communities[$index]['id'],
                $decoded['id'],
                "NDJSON line {$index} ID does not match JSON array"
            );
        }
    }

    public function testFirstNationCount(): void
    {
        $firstNations = array_filter($this->communities, fn($c) => $c['type'] === 'first_nation');
        $this->assertGreaterThanOrEqual(9, count($firstNations), 'Must have at least 9 First Nations');
    }

    public function testMunicipalityCount(): void
    {
        $municipalities = array_filter(
            $this->communities,
            fn($c) => in_array($c['type'], ['town', 'city', 'municipality', 'settlement'])
        );
        $this->assertGreaterThanOrEqual(8, count($municipalities), 'Must have at least 8 municipalities/towns');
    }
}
