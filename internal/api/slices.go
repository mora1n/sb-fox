package api

func uniqueInt64s(ids []int64) []int64 {
	if len(ids) < 2 {
		return ids
	}
	seen := map[int64]bool{}
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}
