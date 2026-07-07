package api

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mora1n/sb-fox/internal/models"
	"github.com/mora1n/sb-fox/internal/store"
)

func (s *Server) decorateProfilesValidation(profiles []*models.Profile, ownerUserID int64, allOwners bool) error {
	if len(profiles) == 0 {
		return nil
	}
	templates, err := s.Store.ListTemplateSummaries(ownerUserID, allOwners)
	if err != nil {
		return err
	}
	nodes, err := s.Store.ListNodes(store.NodeFilter{OwnerUserID: ownerUserID, AllOwners: allOwners, OmitRaw: true})
	if err != nil {
		return err
	}
	nodeGroups, err := s.Store.ListNodeGroups(ownerUserID, allOwners)
	if err != nil {
		return err
	}
	templateIDs := make(map[int64]bool, len(templates))
	for _, template := range templates {
		templateIDs[template.ID] = true
	}
	nodeIDs := make(map[int64]bool, len(nodes))
	for _, node := range nodes {
		nodeIDs[node.ID] = true
	}
	groupMap := make(map[int64]*models.NodeGroup, len(nodeGroups))
	for _, group := range nodeGroups {
		groupMap[group.ID] = group
	}
	for _, profile := range profiles {
		profile.Validation = validateProfileRefs(profile, templateIDs, nodeIDs, groupMap)
	}
	return nil
}

func validateProfileRefs(profile *models.Profile, templateIDs, nodeIDs map[int64]bool, groups map[int64]*models.NodeGroup) *models.ProfileValidation {
	if profile == nil {
		return nil
	}
	validation := &models.ProfileValidation{Valid: true}
	missingNodes := map[int64]bool{}
	missingGroups := map[int64]bool{}
	addNode := func(id int64) {
		if id == 0 || nodeIDs[id] {
			return
		}
		missingNodes[id] = true
	}
	addGroup := func(id int64) {
		if id == 0 {
			return
		}
		group, ok := groups[id]
		if !ok {
			missingGroups[id] = true
			return
		}
		if len(group.NodeIDs) == 0 {
			missingGroups[id] = true
			return
		}
		for _, nodeID := range group.NodeIDs {
			addNode(nodeID)
		}
	}
	if !templateIDs[profile.TemplateID] {
		validation.MissingTemplate = true
	}
	for _, id := range profile.NodeIDs {
		addNode(id)
	}
	for _, id := range profile.NodeGroupIDs {
		addGroup(id)
	}
	opts := parseProfileOptions(profile.Options)
	for _, selection := range opts.GroupSelections {
		for _, id := range selection.NodeIDs {
			addNode(id)
		}
		for _, id := range selection.NodeGroupIDs {
			addGroup(id)
		}
	}
	if opts.AutoCountrySelected != nil {
		for _, id := range opts.AutoCountrySelected.NodeIDs {
			addNode(id)
		}
		for _, id := range opts.AutoCountrySelected.NodeGroupIDs {
			addGroup(id)
		}
	}
	if opts.ChainProxySelected != nil {
		for _, id := range opts.ChainProxySelected.NodeIDs {
			addNode(id)
		}
		for _, id := range opts.ChainProxySelected.NodeGroupIDs {
			addGroup(id)
		}
	}
	validation.MissingNodeIDs = mapKeysSorted(missingNodes)
	validation.MissingNodeGroupIDs = mapKeysSorted(missingGroups)
	validation.Valid = !validation.MissingTemplate && len(validation.MissingNodeIDs) == 0 && len(validation.MissingNodeGroupIDs) == 0
	return validation
}

func mapKeysSorted(values map[int64]bool) []int64 {
	if len(values) == 0 {
		return nil
	}
	out := make([]int64, 0, len(values))
	for id := range values {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func profileValidationError(validation *models.ProfileValidation) error {
	if validation == nil || validation.Valid {
		return nil
	}
	var parts []string
	if validation.MissingTemplate {
		parts = append(parts, "template is missing")
	}
	if len(validation.MissingNodeIDs) > 0 {
		parts = append(parts, fmt.Sprintf("missing nodes: %s", joinInt64s(validation.MissingNodeIDs)))
	}
	if len(validation.MissingNodeGroupIDs) > 0 {
		parts = append(parts, fmt.Sprintf("missing node groups: %s", joinInt64s(validation.MissingNodeGroupIDs)))
	}
	if len(parts) == 0 {
		return fmt.Errorf("profile references are invalid")
	}
	return fmt.Errorf("profile references are invalid: %s", strings.Join(parts, "; "))
}

func joinInt64s(ids []int64) string {
	if len(ids) == 0 {
		return ""
	}
	items := make([]string, 0, len(ids))
	for _, id := range ids {
		items = append(items, fmt.Sprintf("#%d", id))
	}
	return strings.Join(items, ", ")
}
