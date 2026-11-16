package githubsource

import "sync"

// cache memoizes directory listings per URL so repeated UI refreshes do not
// hammer the GitHub API. Entries live until eviction keeps the map bounded.
type cache struct {
	mu       sync.Mutex
	capacity int
	entries  map[string]cacheEntry
	order    []string
}

type cacheEntry struct {
	value *ListContentsResult
}

// listingCache is the shared listing cache.
var listingCache = &cache{capacity: 32, entries: map[string]cacheEntry{}}

// get returns a cached listing.
func (c *cache) get(url string) (*ListContentsResult, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[url]
	if !ok {
		return nil, false
	}
	return entry.value, true
}

// put stores a listing, evicting the oldest entry at capacity.
func (c *cache) put(url string, value *ListContentsResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.entries[url]; !ok && len(c.order) >= c.capacity {
		oldest := c.order[0]
		c.order = c.order[1:]
		delete(c.entries, oldest)
	}
	if _, ok := c.entries[url]; !ok {
		c.order = append(c.order, url)
	}
	c.entries[url] = cacheEntry{value: value}
}

// cachedListImages returns the cached listing when present.
func cachedListImages(url string) (*ListContentsResult, bool) {
	return listingCache.get(url)
}

// storeListing caches a listing.
func storeListing(url string, result *ListContentsResult) {
	listingCache.put(url, result)
}
