# ICP Pilot-Labelling Status (issue #667)

**Purpose.** Durable roadmap for the 53-doc ICP ternary-label pilot. Any Claude Code
session (or human labeller) can resume from this file — it captures what's locked,
what's done, what's in flight, and what's blocking progress.

**Branch.** `claude/wave2-icp-labels-scaffold-667`
**Spec.** `docs/prospect-engine-plan.md` Appendix C; amendment landed via #672.
**Schema.** `classifier/testdata/icp_labels.schema.json` (ternary: strong/partial/none)
**Labels artifact.** `classifier/testdata/icp_labels.yml`
**Scratch log.** `classifier/testdata/icp_labelling_log.md` (gitignored — distills into
`icp_labels.yml` §Methodology at doc-20 checkpoint)

---

## 1. Composition (LOCKED — do not redraw)

53 docs across three segments + two none-buckets:

| Stratum | Count | Breakdown |
|---|---|---|
| `indigenous_channel=strong` | 15 | 6 static org pages + 6 news articles + 3 press releases, pan-Canadian |
| `northern_ontario_industry=strong` | 15 | 7 mining + 3 forestry + 2 energy/industry (OPG hydro, ON Northland) + 3 [re-evaluate at doc 20, see gap #NOI-breakdown] |
| `private_sector_smb=strong` | 10 | 6 `obj_ca` diversified firm-types + 3 `financialpost_com` mid-market M&A + 1 [filler TBD] |
| adjacency-none | 8 | 2 AU/NZ Indigenous + 2 southern-Ontario industry + 2 large-cap Canadian + 2 misc |
| true none/none/none | 5 | weather, sports, international politics, lifestyle, entertainment |

Downsize/upsize rationale (recorded in `icp_labelling_log.md`):
- SMB downsized from 15 → 10 because of corpus scarcity (obj_ca has only 6 docs in
  the entire ES corpus; 1 already used in batch 1 → 5 remain). Non-news prospecting
  channels (LinkedIn, CCAB directory, RFP portals once MERX/Biddingo land) are the
  right remedy — filed as open question in `docs/prospect-engine-plan.md` Appendix
  §Open: corpus coverage for segments with low news density.
- Adjacency-none upsized from 5 → 8 to give AU/NZ Indigenous + southern-Ontario
  industry + large-cap Canadian each a dedicated forcing slot.

---

## 2. Methodology rules captured so far

These recur across batches. Distilled into `icp_labels.yml` §Methodology at doc-20
checkpoint.

1. **Subject-of-story vs object-of-commentary (NOI).** For
   `northern_ontario_industry=strong`, the mining/forestry/energy entity must be the
   story's subject — not the object of commentary from outside the segment. Indigenous
   policy statements about mining are subject=Indigenous entity, object=mining; they
   fire `indigenous_channel=strong`, NOT NOI. (Batch 1 Doc 2 Anishinabek.)
2. **Corridor-broadly interpretation (NOI geography).** NE/NW Ontario includes
   Sault Ste Marie (Tenaris), Sudbury (Vale), Red Lake (Kinross Great Bear), Timmins
   (gold belt), Kapuskasing/Hearst (forestry), Moosonee (ON Northland), Pickle Lake.
   BC/AB mining = `none`, not `partial`. (Batch 1 Doc 8 Copper Mountain forcing
   function.)
3. **Adjacency ≠ partial.** If a doc is *adjacent to* the segment but outside v1
   definition (seeded SaaS vs bootstrapped SMB, Canadian-mining-but-wrong-geography,
   multinational-in-Canada vs mid-market), label `none` with notes explaining the
   adjacency. `partial` is reserved for in-segment weak signals only. (Pre-batch Doc 3
   Lumen.io relabel.)
4. **Indigenous-channel pan-Canadian scope.** `indigenous_channel=strong` is not
   corridor-scoped. ITK (national), MKO (Manitoba), Treaty #3 (NW Ontario),
   Anishinabek (Ontario political union), IndigiNews (BC), APTN (national), Turtle
   Island News (Ontario). Geographic restriction here starves the positives.
5. **AU/NZ Indigenous → adjacency-none.** `indigenous_channel` is Canadian by
   definition in v1. AU (ABC Indigenous, ATSIC), NZ (Waatea) content is
   adjacency-none even when `topics=indigenous` fires. This is why the `topics`
   field is NOT geographically anchored and sector_alignment will need Canadian
   place/institution anchoring (cross-references #668).

---

## 3. Batch 1 — DONE

10 docs labelled in `icp_labels.yml` (entries after the 3 schema examples).

| # | doc_id | Stratum | Source | Title (abbrev) |
|---|---|---|---|---|
| 1 | `b56d3baa…444414bb9c55` | indigenous_channel=strong (page) | mkonation_com | MKO letter to AFN National Chief |
| 2 | `69323ad8…36ff0b834` | indigenous_channel=strong (article) | Turtle Island News | Anishinabek Nation mining commentary |
| 3 | `8628d923…30d0637a5b516fee` | indigenous_channel=strong (PR) | kenoraminerandnews_com | Grand Council Treaty #3 advisory |
| 4 | `3189253d…a6dfa825bcb1` | NOI=strong (mining) | financialpost_com | Vale nickel asset divestiture (Sudbury) |
| 5 | `30f502e0…f52d1e569b50af` | NOI=strong (mining) | financialpost_com | Kinross Great Bear permit (Red Lake) |
| 6 | `e5e5f253…649bb351c7f740` | NOI=strong (energy/industry) | www_elliotlaketoday_com | Tenaris SSM 25-year anniversary |
| 7 | `f99ed78f…92888b2e7983bd096be` | SMB=strong | obj_ca | Brazeau Seller Law — Marcogliese promotion |
| 8 | `871a9bdd…1f0251b8d050` | adjacency-none (NOI geography) | mapleridgenews_com | Copper Mountain BC permit |
| 9 | `2cea69bd…229b18ae3` | adjacency-none (triple-boundary) | www_bramptonguardian_com | Brampton Coca-Cola expansion |
| 10 | `8591cd2d…e0fbce9125` | true none/none/none | Global News | Spring weather feature |

Last `labelled_at: 2026-04-19T15:00:00Z`. All 10 have full `notes` + `labeller_confidence`;
Doc 8 explicitly `low` confidence (partial-vs-none forcing function).

---

## 4. Batch 2 — IN FLIGHT (43 docs, drawn but not yet labelled)

### 4a. Stratum composition + draw queries

**Sourcing mechanism (amended 2026-04-19).** Batch sourcing runs against the MCP
`search_content` tool from within a labelling session. The tool does full-text
search with topic and content-type filters. It does not honour lucene
`source_name:` filters or boolean NOT. Queries are therefore specified as
single-topic or single-term bodies with a small `page_size` (start at 25), and
source filtering happens client-side against the returned facets.

Working pool size per query is capped so a response fits under ~20KB. A query
that returns a 50KB+ pool is re-scoped, not truncated.

Source allow-lists by stratum:

- Corridor mining: `www_timminspress_com`, `www_thesudburystar_com`,
  `financialpost_com`, `www_elliotlaketoday_com`.
- Corridor forestry, NE Ontario: `www_myespanolanow_com`, `www_sudbury_com`.
- AU/NZ Indigenous adjacency: ABC Indigenous, Asia Pacific Report.

Excluded for corridor-intent queries: `www_argusmedia_com` (global trade-press),
Ahead of the Herd (BC adjacency).

Lucene examples in other sections remain valid for direct-ES sessions. They are
not the labelling-session spec.

### 4b. Candidates draft state

The draw executed during the prior session surfaced specific candidate doc_ids for
all 43 slots (verified against ES). **They were not captured in a persisted file
before the session compacted.** Rather than relist potentially-stale IDs from memory,
the recovery path is:

1. Re-run the draw queries in §4a using
   `mcp__North_Cloud__Production___search_content`.
2. Apply exclusions (doc_ids already in batch 1; noisy-topics docs per #668).
3. Surface the resulting 43-slate as a new table (mirror structure of
   `/tmp/first-10-candidates.md`) for user sign-off BEFORE labelling begins.

This is the correct pattern regardless — the user's standing rule is "surface the
full 43 candidates for sign-off before labels are applied." The redraw is cheap
(one parallel-search call) and re-establishes ground truth.

---

## 5. Gap log (explicit open items)

### Gap #SMB-obj-ca-scarcity — BLOCKING composition

**Problem.** `source_name:obj_ca` facet count in the whole ES `classified_content`
index = 6 docs. Batch 1 used 1 (Brazeau Seller / `f99ed78f` / `obj_ca` /
`smb=strong`). Only 5 obj_ca docs remain vs. the 6-slot target. Cannot satisfy
the composition from corpus.

**Options:**
- (a) Accept 5 obj_ca, backfill the 6th SMB slot with a second FP mid-market pick
  (total SMB becomes 5 obj_ca + 4 FP).
- (b) Accept 5 obj_ca, backfill with a trade-press SMB pick (e.g. Northern Ontario
  Business profile of a corridor SMB — crosses into NOI territory, muddies label).
- (c) Backfill with a LinkedIn/CCAB/RFP source if available in corpus (unlikely —
  none of these are ingested yet).
- (d) Drop SMB to 9 total, net composition 52 not 53.

**Recommended.** (a) — preserves firm-type diversity source (FP = M&A stories), keeps
label cleanness, and the 1-doc gap is inside the noise floor for v1 validator
coverage targets.

**Decision required from user.**

### Gap #NOI-breakdown — needs draw execution

**Problem.** The 7-mining slot in NOI=strong was specified only to
Vale/Kinross-level. Need 5 more specific corridor-mining operators.

**Candidates queued (from draw queries in §4a):**
- Glencore Sudbury (nickel, smelter expansion)
- Newmont Porcupine (Timmins gold)
- IAMGOLD Côté (Gogama gold, recent)
- Alamos Young-Davidson (Matachewan gold)
- New Gold Rainy River (NW Ontario gold, near Fort Frances)
- Pan American Silver / Wesdome (backup)

Resolve: run the mining draw query, take top-5 excluding Vale/Kinross already used.

### Gap #NOI-forestry — needs draw execution

**Problem.** 3 forestry slots. Hearst/Kapuskasing and Thunder Bay forestry operators
were not confirmed via top-5 ranking on prior draw.

**Candidates queued:**
- Resolute Forest Products (Thunder Bay pulp/paper)
- GreenFirst Forest Products (Kapuskasing sawmill)
- Domtar (Espanola)

Resolve: run the forestry draw query in §4a.

### Gap #NOI-energy-industry — decision recorded

Per prior methodology discussion: "energy/industry 2" slot = OPG hydro (corridor
stations — Umbata Falls, Rat Rapids, Kapuskasing) + ON Northland rail/transit.
Tenaris SSM already in batch 1. Opportunistic multi-label if a First Nation JV or
IBA-signatory story surfaces.

### Gap #batch-2-draft-persisted — RESOLVED-BY-THIS-FILE

The 43-candidate draft table is the artifact that goes stale fastest. The mitigation
is §4a (draw queries pre-specified) + the §4b instruction to redraw cheaply on
pickup. This file IS the persisted state; a per-doc table is regeneratable and
shouldn't block progress.

---

## 6. Current gate

**Awaiting.** User sign-off on:
1. SMB scarcity decision (§5 Gap #SMB-obj-ca-scarcity — recommend option (a)).
2. Batch 2 43-slate (to be re-surfaced via §4b redraw) before labels are applied.

**Not blocked on.** Schema (#672 merged), plan amendment (#672 merged), Wave 2 CI
validator (#669 — parallel track), noisy-topic-tags issue (#668 — parallel,
doesn't invalidate v1 labels).

---

## 7. Resumable next-step directive

For future-Claude (or future-human) picking up this work:

1. **Read this file + `icp_labels.yml` + `icp_labelling_log.md`** (last is gitignored).
2. **Check issue #667** for user response to the SMB scarcity question.
3. **Re-run the 43-slate draw** per §4a queries. Surface as a new table for sign-off.
4. **On sign-off**, label docs 11–53 in `icp_labels.yml` in 10-doc batches. After
   each batch, surface the batch for review.
5. **At doc 20**: methodology distillation checkpoint. Compress
   `icp_labelling_log.md` rules into `icp_labels.yml` §Methodology (top of file).
   Do NOT defer past doc 20.
6. **At doc 53**: final review + prep for #669 Wave 2 validator consumption.

Cross-refs: #667 (parent), #668 (ES mapping + duplicate-doc_id validator —
parallel), #669 (sector_alignment validator — consumes this labels file), #672
(schema + plan amendment, merged).

---

*Last updated 2026-04-19 by Claude during session continuation after context
compaction. Prior work: batch 1 labelled, schema merged (#672), plan amended,
composition locked at 12/12/9/6/4 for batch 2.*
