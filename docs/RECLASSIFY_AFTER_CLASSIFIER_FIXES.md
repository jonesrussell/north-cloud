# Reclassification and Publisher Reset After Classifier Fixes

After deploying classifier fixes (e.g. content_type URL fallback, crime rules title+body prefix), existing stored documents must be reclassified and the publisher cursor reset so corrected classifications flow to Streetcode.

**North Cloud prod:** jones@northcloud.biz, `/opt/north-cloud`

## Sequence

1. **Deploy classifier** with the fixes (CI/CD or manual).
2. **Reclassify** existing content:
   - All `content_type: "page"` documents with high word count (≥200–300).
   - All `not_crime` documents that already have crime types from ML.
3. **Reset publisher cursor** and clear crime-channel publish history so the publisher re-emits corrected crime articles.
4. **Verify** Streetcode receives crime content (subscriber logs show "Article processed").

## Commands (reference)

- Reset cursor and clear crime history:
  ```bash
  ssh jones@northcloud.biz 'docker exec north-cloud-postgres-publisher-1 psql -U postgres -d publisher -c "
  BEGIN;
  DELETE FROM publish_history WHERE channel_name LIKE '\''crime:%'\'';
  UPDATE publisher_cursor SET last_sort = '\''[]'\'', updated_at = NOW() WHERE id = 1;
  COMMIT;
  "'
  ```
- Restart publisher: `ssh jones@northcloud.biz 'cd /opt/north-cloud && docker compose -f docker-compose.base.yml -f docker-compose.prod.yml restart publisher'`
- Verify Streetcode: `ssh deployer@streetcode.net 'tail -50 .../storage/logs/laravel.log | grep "Article processed"'`

## Reclassification script (run on prod server)

Run on `jones@northcloud.biz` (e.g. in `screen` or `tmux`). Replace `AUTH_PASSWORD` with the real auth password. Uses curl container + port **8070** (classifier listens on 8070; classifier container cannot reach its own localhost).

```bash
# On northcloud.biz
cd /opt/north-cloud
TOKEN=$(docker exec north-cloud-auth-1 wget -qO- "http://localhost:8040/api/v1/auth/login" \
  --post-data='{"username":"admin","password":"AUTH_PASSWORD"}' \
  --header="Content-Type: application/json" 2>/dev/null | python3 -c "import sys,json; print(json.load(sys.stdin).get('token',''))")

docker exec north-cloud-elasticsearch-1 curl -s "http://localhost:9200/*_classified_content/_search" \
  -H "Content-Type: application/json" \
  -d '{"query":{"bool":{"must":[{"exists":{"field":"crime"}},{"term":{"crime.street_crime_relevance":"not_crime"}},{"terms":{"crime.crime_types":["violent_crime","property_crime","drug_crime"]}}]}},"_source":false,"size":500}' \
  | python3 -c "import sys,json; [print(h['_id']) for h in json.load(sys.stdin)['hits']['hits']]" > /tmp/reclassify_ids.txt

while read id; do
  code=$(docker run --rm --network=north-cloud_north-cloud-network -e T="$TOKEN" curlimages/curl:8.1.2 -s -o /dev/null -w "%{http_code}" \
    -X POST -H "Authorization: Bearer $T" "http://classifier:8070/api/v1/classify/reclassify/$id")
  [ "$code" = "200" ] && echo "OK $id" || echo "FAIL $id"
done < /tmp/reclassify_ids.txt
```

Full step-by-step (reclassify API, ES queries, verification) is in `docs/plans/2026-02-09-pipeline-restore-and-classifier-fixes.md` (Tasks 2, 8, 9).
