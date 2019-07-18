package learn_ccache

import (
	"container/list"
	"time"
	"hash/fnv"
	"sync/atomic"
)

type Cache struct {
	*Configuration
	list        *list.List
	size        int64
	buckets     []*bucket
	bucketMask  uint32
	deletables  chan *Item
	promotables chan *Item
	donec       chan struct{}
}

func New(config *Configuration) *Cache {
	c := &Cache{
		list:          list.New(),
		Configuration: config,
		bucketMask:    uint32(config.buckets) - 1,
		buckets:       make([]*bucket, config.buckets),
	}

	for i := 0; i < int(config.buckets); i++ {
		c.buckets[i] = &bucket{
			lookup: make(map[string]*Item),
		}
	}
	c.restart()

	return c
}

func (c *Cache) restart() {
	c.deletables = make(chan *Item, c.deleteBuffer)
	c.promotables = make(chan *Item, c.promoteBuffer)
	c.donec = make(chan struct{})
	//go c.worker()
}

func (c *Cache) bucket(key string) *bucket {
	h := fnv.New32()
	h.Write([]byte(key))
	return c.buckets[h.Sum32()&c.bucketMask]
}

func (c *Cache) promote(item *Item) {
	c.promotables <- item
}

func (c *Cache) set(key string, value interface{}, duration time.Duration) *Item {
	item, existing := c.bucket(key).set(key, value, duration)
	if existing != nil {
		c.deletables <- existing
	}
	c.promote(item)
	return item
}

func (c *Cache) deleteItem(bucket *bucket, item *Item) {
	bucket.delete(item.key) //stop other GETs from getting it
	c.deletables <- item
}

func (c *Cache) doDelete(item *Item) {
	if item.element == nil {
		item.promotions = -2
	} else {
		c.size -= item.size
		if c.onDelete != nil {
			c.onDelete(item)
		}
		c.list.Remove(item.element)
	}
}

func (c *Cache) doPromote(item *Item) bool {
	//already deleted
	if item.promotions == -2 {
		return false
	}
	if item.element != nil { //not a new item
		if item.shouldPromote(c.getsPerPromote) {
			c.list.MoveToFront(item.element)
			item.promotions = 0
		}
		return false
	}

	c.size += item.size
	item.element = c.list.PushFront(item)
	return true
}

func (c *Cache) worker() {
	defer close(c.donec)

	for {
		select {
		case item, ok := <-c.promotables:

			if ok == false {
				goto drain
			}
			if c.doPromote(item) && c.size > c.maxSize {
				c.gc()
			}
		case item := <-c.deletables:
			c.doDelete(item)
		}
	}
	//耗尽
drain:
	for {
		select {
		case item := <-c.deletables:
			c.doDelete(item)
		default:
			close(c.deletables)
			return
		}
	}
}


//做回收
func (c *Cache) gc() {
	//
	element := c.list.Back()
	for i := 0; i < c.itemsToPrune; i++ {
		if element == nil {
			return
		}
		prev := element.Prev()
		item := element.Value.(*Item)

		if c.tracking == false || atomic.LoadInt32(&item.refCount) == 0 {
			c.bucket(item.key).delete(item.key)
			c.size -= item.size
			c.list.Remove(element)
			if c.onDelete != nil {
				c.onDelete(item)
			}
			item.promotions = -2
		}
		element = prev
	}
}

//计数
func (c *Cache) ItemCount() int {
	count := 0
	for _, b := range c.buckets {
		count += b.itemCount()
	}
	return count
}

//从缓存中获取一个项。如果没有找到该项，则返回nil。
//这可以返回过期的项目。使用item. expired()查看项目是否存在
//已过期，然后item. ttl()来查看该条目要多长时间过期(其中对于已经过期的项目将为负数)。
func (c *Cache) Get(key string) *Item {
	item := c.bucket(key).get(key)
	if item == nil {
		return nil
	}
	if item.expires > time.Now().UnixNano() {
		c.promote(item)
	}
	return item
}

//获取之后标记
func (c *Cache) TrackingGet(key string) TrackedItem {
	item := c.Get(key)
	if item == nil {
		return NilTracked
	}
	item.track()
	return item
}

func (c *Cache) Set(key string, value interface{}, duration time.Duration) {
	c.set(key, value, duration)
}

//如果该值存在，则替换该值;如果不存在，则不设置该值。
//如果存在的项被替换，返回true，否则返回false。
// Replace不会重置项目的TTL
func (c *Cache) Replace(key string, value interface{}) bool {
	item := c.bucket(key).get(key)
	if item == nil {
		return false
	}
	c.Set(key, value, item.TTL())
	return true
}

//试图从缓存中获取值，调用在未获取值时获取
//或陈旧的物品。如果fetch返回一个错误，则不缓存任何值，也不缓存该错误
//返回给调用者
func (c *Cache) Fetch(key string, duration time.Duration, fetch func() (interface{}, error)) (*Item, error) {
	item := c.Get(key)
	if item != nil && !item.Expired() {
		return item, nil
	}

	//获取
	value, err := fetch()
	if err != nil {
		return nil, err
	}
	return c.set(key, value, duration), nil
}

func (c *Cache) Delete(key string) bool {
	item := c.bucket(key).delete(key)
	if item != nil {
		c.deletables <- item
		return true
	}
	return false
}

func (c *Cache) Clear() {
	for _, bucket := range c.buckets {
		bucket.clear()
	}
	c.size = 0
	c.list = list.New()
}

func (c *Cache) Stop() {
	close(c.promotables)
	<-c.donec
}
