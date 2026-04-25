UPDATE sources
SET disabled_at = COALESCE(disabled_at, updated_at, NOW()),
    disable_reason = 'legacy_unknown'
WHERE enabled = false
  AND disable_reason IS NULL;
