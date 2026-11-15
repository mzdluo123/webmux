package workspace

import (
	"bytes"
	"context"
	"strconv"
	"strings"
)

type rawHistoryRow struct {
	oid           string
	parents       []string
	subject       string
	authorName    string
	authorEmail   string
	authorAt      string
	committerName string
	committerAt   string
}

func buildHistoryLogArgs(include, exclude []string, q HistoryQuery, cleanPath string, offset, maxCount int, withParents bool, format string) []string {
	args := []string{"log", "--topo-order", "--no-show-signature", "--no-ext-diff", "--no-textconv"}
	if withParents {
		args = append(args, "--parents")
	}
	if offset > 0 {
		args = append(args, "--skip="+strconv.Itoa(offset))
	}
	if maxCount > 0 {
		args = append(args, "-n", strconv.Itoa(maxCount))
	}
	if q.Query != "" {
		args = append(args, "--fixed-strings", "--regexp-ignore-case", "--grep="+q.Query)
	}
	if q.Author != "" {
		args = append(args, "--fixed-strings", "--regexp-ignore-case", "--author="+q.Author)
	}
	if q.Since != "" {
		args = append(args, "--since="+q.Since)
	}
	if q.Until != "" {
		args = append(args, "--until="+q.Until)
	}
	args = append(args, "--format="+format)
	args = append(args, include...)
	for _, ex := range exclude {
		args = append(args, "^"+ex)
	}
	args = append(args, "--")
	if cleanPath != "" {
		args = append(args, cleanPath)
	}
	return args
}

func queryHistoryRows(ctx context.Context, anchor string, include, exclude []string, q HistoryQuery, cleanPath string, offset, limit int) ([]rawHistoryRow, bool, error) {
	format := "%H%x00%P%x00%s%x00%an%x00%ae%x00%aI%x00%cn%x00%cI%x00"
	args := buildHistoryLogArgs(include, exclude, q, cleanPath, offset, limit+1, false, format)
	data, err := ReadGit(ctx, anchor, maxHistoryOutput, args...)
	if err != nil {
		return nil, false, err
	}
	fields := bytes.Split(data, []byte{0})
	const stride = 8
	rows := make([]rawHistoryRow, 0, limit+1)
	for i := 0; i+stride-1 < len(fields); i += stride {
		oid := strings.TrimPrefix(string(fields[i]), "\n")
		if !isObjectID(oid) {
			continue
		}
		var parents []string
		if rawParents := strings.TrimSpace(string(fields[i+1])); rawParents != "" {
			parents = strings.Fields(rawParents)
		} else {
			parents = []string{}
		}
		rows = append(rows, rawHistoryRow{
			oid:           oid,
			parents:       parents,
			subject:       string(fields[i+2]),
			authorName:    string(fields[i+3]),
			authorEmail:   string(fields[i+4]),
			authorAt:      string(fields[i+5]),
			committerName: string(fields[i+6]),
			committerAt:   string(fields[i+7]),
		})
	}
	hasMore := false
	if len(rows) > limit {
		hasMore = true
		rows = rows[:limit]
	}
	return rows, hasMore, nil
}

func queryRewrittenParents(ctx context.Context, anchor string, include, exclude []string, q HistoryQuery, cleanPath string, offset, limit int) (map[string][]string, error) {
	format := "%H%x00%P%x00"
	args := buildHistoryLogArgs(include, exclude, q, cleanPath, offset, limit, true, format)
	data, err := ReadGit(ctx, anchor, maxHistoryOutput, args...)
	if err != nil {
		return nil, err
	}
	fields := bytes.Split(data, []byte{0})
	out := make(map[string][]string, limit)
	for i := 0; i+1 < len(fields); i += 2 {
		oid := strings.TrimPrefix(string(fields[i]), "\n")
		if !isObjectID(oid) {
			continue
		}
		if raw := strings.TrimSpace(string(fields[i+1])); raw != "" {
			out[oid] = strings.Fields(raw)
		}
	}
	return out, nil
}

// layoutHistoryGraph assigns deterministic lanes and edge kinds ("direct",
// "indirect", "boundary") across paginated topo-ordered commit rows.
//
// Invariants:
//   - "direct" edges ONLY exist when targetOID is a real parent of row.OID AND
//     targetOID is present in the current page.
//   - When filtering omits intermediate commits so a rewritten visible ancestor A
//     is NOT in row.Parents, an "indirect" edge is emitted to A (never "direct"),
//     and DirectParentsMissing is set true.
//   - When a real parent (or rewritten ancestor) lies on a subsequent page, its
//     lane stays open at the bottom of the page and a "boundary" edge carries its
//     real OID into nextLanes so page N+1 continues the exact lane.
func layoutHistoryGraph(raw []rawHistoryRow, initialLanes []string, refsMap map[string][]HistoryRefBadge, rewritten map[string][]string) ([]HistoryRow, []string) {
	inPage := make(map[string]bool, len(raw))
	for _, r := range raw {
		inPage[r.oid] = true
	}

	lanes := append([]string(nil), initialLanes...)
	out := make([]HistoryRow, 0, len(raw))

	for _, r := range raw {
		// Find all lanes currently expecting r.oid.
		lane := -1
		var converging []int
		for idx, target := range lanes {
			if target == r.oid {
				if lane == -1 {
					lane = idx
				} else {
					converging = append(converging, idx)
				}
			}
		}
		if lane == -1 {
			lane = allocateLane(&lanes, r.oid)
		}
		for _, idx := range converging {
			lanes[idx] = ""
		}

		realParentSet := make(map[string]bool, len(r.parents))
		for _, p := range r.parents {
			realParentSet[p] = true
		}

		// Determine outgoing graph targets for this row.
		type graphTarget struct {
			oid    string
			direct bool
		}
		var targets []graphTarget
		directMissing := false

		if rewritten != nil {
			simp := rewritten[r.oid]
			simpSet := make(map[string]bool, len(simp))
			for _, anc := range simp {
				simpSet[anc] = true
			}
			for _, p := range r.parents {
				if !simpSet[p] {
					directMissing = true
				}
			}
			if len(simp) > 0 {
				seenTarget := map[string]bool{}
				for _, anc := range simp {
					if seenTarget[anc] {
						continue
					}
					seenTarget[anc] = true
					isDirect := realParentSet[anc]
					if !isDirect {
						directMissing = true
					}
					targets = append(targets, graphTarget{oid: anc, direct: isDirect})
				}
			}
		} else {
			for _, p := range r.parents {
				targets = append(targets, graphTarget{oid: p, direct: true})
			}
		}

		edges := make([]HistoryEdge, 0, len(targets))
		if len(targets) == 0 {
			lanes[lane] = ""
		} else {
			// Place first target on current lane (unless another lane already tracks it).
			first := targets[0]
			existingFirst := findLane(lanes, first.oid, lane)
			toFirst := lane
			if existingFirst != -1 {
				lanes[lane] = ""
				toFirst = existingFirst
			} else {
				lanes[lane] = first.oid
			}
			edges = append(edges, HistoryEdge{
				FromLane:  lane,
				ToLane:    toFirst,
				TargetOID: first.oid,
				Kind:      classifyEdgeKind(first.direct, inPage[first.oid]),
			})

			for _, extra := range targets[1:] {
				toIdx := findLane(lanes, extra.oid, -1)
				if toIdx == -1 {
					toIdx = allocateLane(&lanes, extra.oid)
				}
				edges = append(edges, HistoryEdge{
					FromLane:  lane,
					ToLane:    toIdx,
					TargetOID: extra.oid,
					Kind:      classifyEdgeKind(extra.direct, inPage[extra.oid]),
				})
			}
		}

		trimTrailingEmptyLanes(&lanes)
		active := make([]int, 0, len(lanes))
		for idx, target := range lanes {
			if target != "" || idx == lane {
				active = append(active, idx)
			}
		}

		shortOID := r.oid
		if len(shortOID) > 8 {
			shortOID = shortOID[:8]
		}
		badges := refsMap[r.oid]
		if badges == nil {
			badges = []HistoryRefBadge{}
		}
		out = append(out, HistoryRow{
			OID:                  r.oid,
			ShortOID:             shortOID,
			Parents:              r.parents,
			Subject:              r.subject,
			AuthorName:           r.authorName,
			AuthorEmail:          r.authorEmail,
			AuthorAt:             r.authorAt,
			CommitterName:        r.committerName,
			CommitterAt:          r.committerAt,
			Refs:                 badges,
			Lane:                 lane,
			ActiveLanes:          active,
			Edges:                edges,
			DirectParentsMissing: directMissing,
		})
	}

	trimTrailingEmptyLanes(&lanes)
	return out, lanes
}

func classifyEdgeKind(isDirectParent, inCurrentPage bool) string {
	if !isDirectParent {
		return "indirect"
	}
	if !inCurrentPage {
		return "boundary"
	}
	return "direct"
}

func findLane(lanes []string, oid string, skipIdx int) int {
	for i, v := range lanes {
		if i != skipIdx && v == oid {
			return i
		}
	}
	return -1
}

func allocateLane(lanes *[]string, oid string) int {
	for i, v := range *lanes {
		if v == "" {
			(*lanes)[i] = oid
			return i
		}
	}
	*lanes = append(*lanes, oid)
	return len(*lanes) - 1
}

func trimTrailingEmptyLanes(lanes *[]string) {
	s := *lanes
	for len(s) > 0 && s[len(s)-1] == "" {
		s = s[:len(s)-1]
	}
	*lanes = s
}
