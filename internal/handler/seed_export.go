package handler

import "github.com/LaPingvino/esperanto-kurso-gae/internal/model"

// SeedGroups returns all built-in seed items grouped by filename (without .json).
// Used by cmd/export-seed to write seed/*.json files.
func SeedGroups() map[string][]*model.ContentItem {
	return map[string][]*model.ContentItem{
		"zagr-vortaro":  seedItems(),
		"zagr-enhavo":   append(seedContentItems(), seedFillinItems()...),
		"video":         seedVideoItems(),
		"ekstra":        seedExtraItems(),
	}
}
