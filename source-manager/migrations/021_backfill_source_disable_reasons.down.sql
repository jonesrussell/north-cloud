UPDATE sources
SET disabled_at = NULL,
    disable_reason = NULL
WHERE enabled = false
  AND disable_reason = 'legacy_unknown';
