ALTER TABLE communities
    ADD COLUMN osm_relation_id BIGINT,
    ADD COLUMN wikidata_qid    VARCHAR(20);

CREATE INDEX idx_communities_osm_relation_id
    ON communities(osm_relation_id) WHERE osm_relation_id IS NOT NULL;

CREATE INDEX idx_communities_wikidata_qid
    ON communities(wikidata_qid) WHERE wikidata_qid IS NOT NULL;
