package tray

// ServerItem is one entry in the tray's server submenu.
//
// It carries only what the menu needs to render and act: the id to hand back,
// the label to show, and whether the entry is pinned. It lives in a
// platform-neutral file so the app can build the list on every platform, even
// though only Windows currently renders it.
type ServerItem struct {
	ID       string
	Name     string
	Favorite bool
}
