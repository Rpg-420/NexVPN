package config

import (
	"fmt"
	"time"
)

// SubscriptionChange summarises what one subscription refresh changed.
//
// A refresh used to be invisible: the daemon replaced a group's profiles and
// the user saw a list that was simply different. That is fine when a
// subscription gains servers and bad when it quietly loses the one that was
// working, because there is nothing to look at afterwards — the old rows are
// gone. Recording the delta is what makes the second case diagnosable.
type SubscriptionChange struct {
	URL   string `json:"url"`
	Group string `json:"group"`

	Added     int `json:"added"`
	Removed   int `json:"removed"`
	Renamed   int `json:"renamed"`
	Unchanged int `json:"unchanged"`

	// The names are capped so that a subscription swapping its entire server
	// list cannot turn this record into a second copy of the profile table.
	AddedNames   []string `json:"added_names"`
	RemovedNames []string `json:"removed_names"`
	RenamedNames []string `json:"renamed_names"`

	At time.Time `json:"at"`
}

// subscriptionChangeNameCap bounds each name list in a SubscriptionChange.
const subscriptionChangeNameCap = 25

// subscriptionChangeHistory is how many refreshes are kept. The interesting one
// is always the most recent, and this is a convenience record rather than an
// audit log.
const subscriptionChangeHistory = 20

// profileIdentity returns the key used to decide whether two profile rows
// describe the same node.
//
// The raw URI is the strongest signal when present: it is exactly what the
// provider published. Server+port+protocol is the fallback for rows built from
// a config file rather than a URI, and the name is the last resort. Using the
// name first would be wrong — providers renumber names constantly, so every
// refresh would look like a full replacement.
func profileIdentity(p Profile) string {
	if p.RawURI != "" {
		return "uri:" + p.RawURI
	}
	if p.Server != "" && p.Port > 0 {
		return fmt.Sprintf("addr:%s:%d/%s", p.Server, p.Port, p.Protocol)
	}
	return "name:" + p.Name
}

// recordSubscriptionChangeLocked compares the rows being replaced with the rows
// arriving, and stores the summary. The caller must hold s.mu.
func (s *Store) recordSubscriptionChangeLocked(subURL string, previous, incoming []Profile) {
	// A first import has nothing to compare against, and reporting "142 added"
	// for it would be noise rather than information.
	if len(previous) == 0 {
		return
	}

	previousByIdentity := make(map[string]Profile, len(previous))
	for _, p := range previous {
		previousByIdentity[profileIdentity(p)] = p
	}
	incomingIdentities := make(map[string]struct{}, len(incoming))
	for _, p := range incoming {
		incomingIdentities[profileIdentity(p)] = struct{}{}
	}

	change := SubscriptionChange{URL: subURL, At: time.Now()}

	group := ""
	for _, p := range incoming {
		if p.Group != "" {
			group = p.Group
			break
		}
	}
	if group == "" {
		for _, p := range previous {
			if p.Group != "" {
				group = p.Group
				break
			}
		}
	}
	change.Group = group

	for _, p := range incoming {
		identity := profileIdentity(p)
		old, existed := previousByIdentity[identity]
		if !existed {
			change.Added++
			if len(change.AddedNames) < subscriptionChangeNameCap {
				change.AddedNames = append(change.AddedNames, p.Name)
			}
			continue
		}
		if old.Name != p.Name {
			change.Renamed++
			if len(change.RenamedNames) < subscriptionChangeNameCap {
				change.RenamedNames = append(change.RenamedNames, old.Name+" → "+p.Name)
			}
			continue
		}
		change.Unchanged++
	}

	for _, p := range previous {
		if _, stillThere := incomingIdentities[profileIdentity(p)]; stillThere {
			continue
		}
		change.Removed++
		if len(change.RemovedNames) < subscriptionChangeNameCap {
			change.RemovedNames = append(change.RemovedNames, p.Name)
		}
	}

	// Nothing changed: recording an empty delta would push the useful entry out
	// of the history for no reason.
	if change.Added == 0 && change.Removed == 0 && change.Renamed == 0 {
		return
	}

	s.subscriptionChanges = append(s.subscriptionChanges, change)
	if len(s.subscriptionChanges) > subscriptionChangeHistory {
		s.subscriptionChanges = s.subscriptionChanges[len(s.subscriptionChanges)-subscriptionChangeHistory:]
	}
}

// GetSubscriptionChanges returns the recorded refresh deltas, most recent last.
func (s *Store) GetSubscriptionChanges() []SubscriptionChange {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]SubscriptionChange, len(s.subscriptionChanges))
	copy(out, s.subscriptionChanges)
	return out
}

// ClearSubscriptionChanges discards the recorded deltas.
func (s *Store) ClearSubscriptionChanges() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subscriptionChanges = nil
}
