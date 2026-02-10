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

Full step-by-step (reclassify API, ES queries, verification) is in `docs/plans/2026-02-09-pipeline-restore-and-classifier-fixes.md` (Tasks 2, 8, 9).
