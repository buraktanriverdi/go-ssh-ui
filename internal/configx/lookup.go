package configx

import "go-ssh/config"

// FindHostByID searches the tree for a host with the given stable ID.
func FindHostByID(cfg *config.Config, id string) (config.Host, bool) {
	return findHostByID(cfg.Categories, id)
}

func findHostByID(categories []config.Category, id string) (config.Host, bool) {
	for _, cat := range categories {
		for _, h := range cat.Hosts {
			if h.ID == id {
				return h, true
			}
		}
		if h, ok := findHostByID(cat.Categories, id); ok {
			return h, true
		}
	}
	return config.Host{}, false
}

// FindHostByPath resolves a host given its category path and name - the
// same addressing the host tree UI already uses for edit/delete, and it
// works even for hosts that don't have a stable ID assigned yet (i.e. every
// host that was never saved through go-ssh-ui's own forms).
func FindHostByPath(cfg *config.Config, categoryPath []string, name string) (config.Host, bool) {
	cat, ok := findCategoryByPath(cfg.Categories, categoryPath)
	if !ok {
		return config.Host{}, false
	}
	for _, h := range cat.Hosts {
		if h.Name == name {
			return h, true
		}
	}
	return config.Host{}, false
}

func findCategoryByPath(categories []config.Category, path []string) (config.Category, bool) {
	if len(path) == 0 {
		return config.Category{}, false
	}
	for _, cat := range categories {
		if cat.Name == path[0] {
			if len(path) == 1 {
				return cat, true
			}
			return findCategoryByPath(cat.Categories, path[1:])
		}
	}
	return config.Category{}, false
}
