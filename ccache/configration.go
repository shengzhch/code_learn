package learn_ccache

type Configuration struct {
	maxSize        int64
	buckets        int
	itemsToPrune   int
	deleteBuffer   int
	promoteBuffer  int
	getsPerPromote int32
	tracking       bool
	onDelete       func(item *Item)
}

func Configure() *Configuration {
	return &Configuration{
		buckets:        16,
		itemsToPrune:   500,
		deleteBuffer:   1024,
		getsPerPromote: 3,
		promoteBuffer:  1024,
		maxSize:        5000,
		tracking:       false,
	}
}

func (c *Configuration) MaxSize(max int64) *Configuration {
	c.maxSize = max
	return c
}

// Must be a power of 2 (1, 2, 4, 8, 16, ...)
func (c *Configuration) Buckets(count uint32) *Configuration {
	if count == 0 || ((count & (^count + 1)) == count) == false {
		count = 16
	}
	c.buckets = int(count)
	return c
}


//The number of items to prune when memory is low
func (c *Configuration) ItemsToPrune(count uint32) *Configuration {
	c.itemsToPrune = int(count)
	return c
}

//应该提升的项的队列大小。如果队列已满，则跳过促销
func (c *Configuration) PromoteBuffer(size uint32) *Configuration {
	c.promoteBuffer = int(size)
	return c
}


//应该删除的项的队列大小。如果队列满了，对Delete()的调用将阻塞
func (c *Configuration) DeleteBuffer(size uint32) *Configuration {
	c.deleteBuffer = int(size)
	return c
}

//提供一个高读/写比的大缓存，通常是不必要的在每个Get时去提升一个项目。
//GetsPerPromote指定在提升密钥之前必须拥有的get数量。（几次访问后 把改项目放到开头）
func (c *Configuration) GetsPerPromote(count int32) *Configuration {
	c.getsPerPromote = count
	return c
}

func (c *Configuration) Track() *Configuration {
	c.tracking = true
	return c
}

func (c *Configuration) OnDelete(callback func(item *Item)) *Configuration {
	c.onDelete = callback
	return c
}
