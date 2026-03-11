DROP INDEX IF EXISTS idx_communities_wikidata_qid;
DROP INDEX IF EXISTS idx_communities_osm_relation_id;

ALTER TABLE communities
    DROP COLUMN IF EXISTS wikidata_qid,
    DROP COLUMN IF EXISTS osm_relation_id;
