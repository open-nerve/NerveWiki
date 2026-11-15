package app

import "uuid"

// HubCounts is how many credentials and accounts h counts streams of: none
// once every stream closed (M12/P3 design review B-N10).
func HubCounts(h *Hub) (credentials, accounts int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.held), len(h.accounts)
}

// AccountStreams is how many streams h counts of the account userID.
func AccountStreams(h *Hub, userID uuid.UUID) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.accounts[userID]
}
