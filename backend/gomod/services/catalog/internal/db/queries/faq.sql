-- FAQ reads for the public FAQ endpoints used by the shopping assistant.

-- name: ListFAQs :many
-- Free-text search over question+answer. The assistant forwards whatever the
-- model wrote, which is often a full sentence ("return policy how many days to
-- return"). websearch_to_tsquery ANDs its terms, so that sentence matched zero
-- rows and the assistant answered "no policy information".
-- $2 is the same text rewritten as an OR-tsquery by repository.toTSQuery; the
-- ILIKE arms keep plain partial-word searches ("return" vs "returns") working.
SELECT faq_id, faq_uuid, question, answer, tags, is_active, faq_created_at, faq_updated_at
FROM catalog_faqs
WHERE is_active = TRUE
  AND (sqlc.narg('q')::text IS NULL OR (
    to_tsvector('english', question || ' ' || answer) @@ to_tsquery('english', sqlc.narg('tsq')::text)
    OR question ILIKE '%' || sqlc.narg('q')::text || '%'
    OR answer ILIKE '%' || sqlc.narg('q')::text || '%'
  ))
ORDER BY ts_rank(to_tsvector('english', question || ' ' || answer), to_tsquery('english', sqlc.narg('tsq')::text)) DESC, faq_created_at ASC
LIMIT sqlc.arg('lim') OFFSET sqlc.arg('off');

-- name: ListAllFAQs :many
SELECT faq_id, faq_uuid, question, answer, tags, is_active, faq_created_at, faq_updated_at
FROM catalog_faqs
WHERE is_active = TRUE
ORDER BY faq_created_at ASC;
