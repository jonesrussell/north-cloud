<?php

namespace Database\Seeders;

use Illuminate\Database\Seeder;
use Illuminate\Support\Facades\DB;

class CommunitySeeder extends Seeder
{
    /**
     * Seed the communities table from the canonical communities.json dataset.
     */
    public function run(): void
    {
        $jsonPath = base_path('data/communities/communities.json');

        if (!file_exists($jsonPath)) {
            $this->command->error("Community data file not found: {$jsonPath}");
            return;
        }

        $json = file_get_contents($jsonPath);
        $communities = json_decode($json, true);

        if (json_last_error() !== JSON_ERROR_NONE) {
            $this->command->error('Failed to parse communities.json: ' . json_last_error_msg());
            return;
        }

        $this->command->info("Seeding " . count($communities) . " communities...");

        foreach ($communities as $community) {
            DB::table('communities')->updateOrInsert(
                ['id' => $community['id']],
                [
                    'name'           => $community['name'],
                    'type'           => $community['type'],
                    'province'       => $community['province'],
                    'latitude'       => $community['latitude'],
                    'longitude'      => $community['longitude'],
                    'population'     => $community['population'] ?? null,
                    'governing_body' => $community['governing_body'] ?? null,
                    'external_ids'   => json_encode($community['external_ids'] ?? []),
                    'region'         => $community['region'] ?? null,
                    'neighbours'     => json_encode($community['neighbours'] ?? []),
                    'notes'          => $community['notes'] ?? null,
                    'created_at'     => now(),
                    'updated_at'     => now(),
                ]
            );
        }

        $this->command->info("Seeded " . count($communities) . " communities successfully.");
    }
}
