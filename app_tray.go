package main

import (
	"sort"

	"nexvpn/internal/tray"
)

// trayServerMenuLimit caps how many profiles the tray submenu shows.
//
// A tray menu is scanned, not read. Beyond about a dozen entries the user is
// hunting rather than choosing, and the useful subset — pinned servers and
// whatever they used last — is always near the top anyway.
const trayServerMenuLimit = 12

// trayServerItems builds the tray's server submenu: pinned profiles first, then
// the most recently used.
//
// Pinned first is the whole point of pinning, and recency is the right tiebreak
// because the server someone used last is the one they are most likely to want
// again.
func (a *App) trayServerItems() []tray.ServerItem {
	profiles := a.store.GetProfiles()
	if len(profiles) == 0 {
		return nil
	}

	// GetProfiles returns a copy, so this sort cannot reorder the store.
	sort.SliceStable(profiles, func(i, j int) bool {
		if profiles[i].Favorite != profiles[j].Favorite {
			return profiles[i].Favorite
		}
		return profiles[i].LastUsed.After(profiles[j].LastUsed)
	})

	if len(profiles) > trayServerMenuLimit {
		profiles = profiles[:trayServerMenuLimit]
	}

	items := make([]tray.ServerItem, 0, len(profiles))
	for _, profile := range profiles {
		items = append(items, tray.ServerItem{
			ID:       profile.ID,
			Name:     profile.Name,
			Favorite: profile.Favorite,
		})
	}
	return items
}

// SetProfileFavorite pins or unpins a profile from the UI.
func (a *App) SetProfileFavorite(profileID string, favorite bool) error {
	if err := a.store.SetProfileFavorite(profileID, favorite); err != nil {
		return err
	}
	if favorite {
		a.engine.AddLog("info", "Pinned to the tray menu", "app")
	}
	return nil
}
