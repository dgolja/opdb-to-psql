-- Reconstructs the original OPDB JSON shape from the normalized tables,
-- so it can be served through PostgREST.

CREATE VIEW opdb_export WITH (security_invoker = true) AS
SELECT
  o.opdb_id AS "opdbId",
  o.opdb_group AS "opdbGroup",
  o.opdb_machine AS "opdbMachine",
  o.name,
  o.short_name AS "shortName",
  o.common_name AS "commonName",
  o.name_sort AS "nameSort",
  o.year,
  o.manufacture_date AS "manufactureDate",
  o.description,
  o.type,
  o.display,
  o.player_count AS "playerCount",
  o.physical_machine AS "physicalMachine",
  o.manufacturer_id AS "manufacturerId",
  o.ipdb_id AS "ipdbId",
  o.pinball_primer_url AS "pinballPrimerUrl",
  o.pinball_rules_url AS "pinballRulesUrl",
  o.pinball_cards_url AS "pinballCardsUrl",
  o.bobs_guide_url AS "bobsGuideUrl",
  o.competition_setup_url AS "competitionSetupUrl",
  o.competition_notes_url AS "competitionNotesUrl",
  o.has_competition_notes AS "hasCompetitionNotes",
  o.has_competition_setup AS "hasCompetitionSetup",
  to_char(o.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"') AS "createdAt",
  to_char(o.updated_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"') AS "updatedAt",
  o.entry_type AS "entryType",
  (
    SELECT json_build_object(
             'manufacturerId', m.manufacturer_id,
             'name', m.name,
             'fullName', m.full_name
           )
    FROM manufacturers m
    WHERE m.manufacturer_id = o.manufacturer_id
  ) AS manufacturer,
  COALESCE(
    (
      SELECT json_agg(
               json_build_object(
                 'opdbPersonId', p.opdb_person_id,
                 'name', p.name,
                 'role', op.role_name,
                 'index', op.person_index
               )
               ORDER BY op.person_index
             )
      FROM opdb_people op
      JOIN people p ON p.opdb_person_id = op.opdb_person_id
      WHERE op.opdb_id = o.opdb_id
    ),
    '[]'::json
  ) AS people,
  COALESCE(
    (
      SELECT json_agg(
               json_build_object(
                 'group', i.group_id,
                 'title', i.title,
                 'primary', i.is_primary,
                 'type', i.type,
                 'urls', v.urls,
                 'sizes', v.sizes
               )
               ORDER BY i.is_primary DESC, i.group_id
             )
      FROM opdb_images oi
      JOIN images i ON i.group_id = oi.group_id
      JOIN LATERAL (
        SELECT
          json_object_agg(iv.variant, iv.url) AS urls,
          json_object_agg(iv.variant, json_build_object('width', iv.width, 'height', iv.height)) AS sizes
        FROM image_variants iv
        WHERE iv.group_id = i.group_id
      ) v ON true
      WHERE oi.opdb_id = o.opdb_id
    ),
    '[]'::json
  ) AS images,
  COALESCE(
    (
      SELECT json_agg(
               json_build_object(
                 'featureId', f.feature_id,
                 'name', f.name,
                 'group', f.group_name
               )
               ORDER BY f.feature_id
             )
      FROM opdb_features ofe
      JOIN features f ON f.feature_id = ofe.feature_id
      WHERE ofe.opdb_id = o.opdb_id
    ),
    '[]'::json
  ) AS features,
  COALESCE(o.keywords, ARRAY[]::text[]) AS keywords
FROM opdb o;

-- Wraps the view in the source file's top-level {"entries": [...]} envelope
-- and exposes it as a PostgREST RPC endpoint: GET /rpc/opdb_entries
CREATE FUNCTION opdb_entries()
RETURNS json
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = ''
AS $$
  SELECT json_build_object('entries', COALESCE(json_agg(e), '[]'::json))
  FROM public.opdb_export e;
$$;

-- Wraps the singel view value in the source file's top-level {"data": {...}}
-- envelope and exposes it as a PostgREST RPC endpoint: GET /rpc/opdb_entry
CREATE FUNCTION opdb_entry(
  opdbId text,
  includePeople bool DEFAULT false,
  includeImages bool DEFAULT false
)
RETURNS json
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = ''
AS $$
  SELECT json_build_object(
    'data',
    (to_jsonb(e) - 'people' - 'images')
    || jsonb_build_object(
         'people', CASE WHEN includePeople THEN to_jsonb(e) -> 'people' ELSE '[]'::jsonb END,
         'images', CASE WHEN includeImages THEN to_jsonb(e) -> 'images' ELSE '[]'::jsonb END
       )
  )
  FROM public.opdb_export e
  WHERE "opdbId" = opdbId;
$$;
