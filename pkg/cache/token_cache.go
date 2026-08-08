package cache

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/jumpserver-dev/sdk-go/model"
	"github.com/jumpserver/koko/pkg/logger"
)

/*
Cache the ConnectToken:
	The key is created by binding token_id and addr together
	1. Retrieve the cached ConnectToken by key
	2. A newly requested ConnectToken is added to the cache; if it already exists, its reference count is incremented
	3. Periodically reclaim ConnectTokens from the cache to ensure memory doesn't grow unbounded
		Cleanup conditions:
			1. Reference count is 0 and it has been idle for over five minutes
			2. Cached for over 1 hour
*/

var TokenCacheInstance = NewConnectTokenCache()

func NewConnectTokenCache() *ConnectTokenCache {
	cache := &ConnectTokenCache{
		data: make(map[string]*ConnectTokenItem),
	}
	go cache.run()
	return cache
}

type ConnectTokenCache struct {
	lock sync.Mutex
	data map[string]*ConnectTokenItem
}

func (c *ConnectTokenCache) Get(key string) *model.ConnectToken {
	c.lock.Lock()
	defer c.lock.Unlock()
	item, ok := c.data[key]
	if !ok {
		return nil
	}
	return item.token
}

func (c *ConnectTokenCache) Save(key string, token *model.ConnectToken) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if item, ok := c.data[key]; ok {
		item.refInc()
		logger.Infof("Cache key %s ref: %d", key, item.Ref)
		return
	}
	item := NewConnectTokenItem(token)
	c.data[key] = item
	logger.Infof("New cache key %s ref: %d", key, item.Ref)
}

func (c *ConnectTokenCache) Recycle(key string) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if item, ok := c.data[key]; ok {
		item.refDec()
		logger.Infof("Recycle cache key %s ref: %d", key, item.Ref)
	} else {
		logger.Warnf("Recycle cache key %s not found", key)
	}
}

func (c *ConnectTokenCache) run() {
	for {
		c.GC()
		time.Sleep(30 * time.Second)
	}
}

func (c *ConnectTokenCache) GC() {
	readyDelete := make([]string, 0, len(c.data))
	c.lock.Lock()
	now := time.Now()
	for k, v := range c.data {
		if v.IsExpired(now) {
			readyDelete = append(readyDelete, k)
		}
	}
	for _, k := range readyDelete {
		delete(c.data, k)
		logger.Infof("ConnectToken %s is expired, recycled", k)
	}
	c.lock.Unlock()
}

func NewConnectTokenItem(token *model.ConnectToken) *ConnectTokenItem {
	now := time.Now()
	return &ConnectTokenItem{
		token:          token,
		Ref:            1,
		expiredTime:    now.Add(5 * time.Minute).Unix(),
		maxExpiredTime: now.Add(time.Hour).Unix(),
	}
}

type ConnectTokenItem struct {
	token          *model.ConnectToken
	Ref            uint
	expiredTime    int64
	maxExpiredTime int64
}

func (c *ConnectTokenItem) IsExpired(now time.Time) bool {
	if c.maxExpiredTime < now.Unix() {
		return true
	}
	return c.Ref == 0 && c.expiredTime < now.Unix()
}

func (c *ConnectTokenItem) refDec() {
	c.Ref--
	c.expiredTime = time.Now().Add(5 * time.Minute).Unix()
}

func (c *ConnectTokenItem) refInc() {
	c.Ref++
	c.expiredTime = time.Now().Add(5 * time.Minute).Unix()
}

// Bind a key to an addr

func CreateAddrCacheKey(addr net.Addr, token string) string {
	ip, _, _ := net.SplitHostPort(addr.String())
	return fmt.Sprintf("%s-%s", ip, token)
}
