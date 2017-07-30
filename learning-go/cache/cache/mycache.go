package cache

import (
    "encoding/gob"
    "fmt"
    "io"
    "os"
    "sync"
    "time"
)

type Item struct{
	Object interface{}
	Expr int64 //生存时间
}

func (item Item) Expired() bool{
	if item.Expr == 0 {
		return false
	}
	return time.Now().UnixNano() > item.Expr
}

const(
	NoExpr time.Duration = -1 //没有过期时间标志
	DefaultExpiration time.Duration = 0//默认过期时间
)

type Cache struct{
	defaultExp time.Duration
	items      map[string]Item
	mu      sync.RWMutex
	gcInterval time.Duration
	stopGc     chan bool
}

func (c *Cache) gcLoop(){
	ticker := time.NewTicker(c.gcInterval)
	for{
		select{
			case <-ticker.C:
				c.DeleteExpired()
			case <-c.stopGc:
				ticker.Stop()
				return
		}
	}
}

func (c *Cache) delete(k string){
	delete(c.items, k)
}

func (c *Cache) DeleteExpired(){
	now := time.Now().UnixNano()
	c.mu.Lock()
	defer c.mu.Unlock()

	for k, v := range c.items{
		if v.Expr > 0 && now > v.Expr {
			c.delete(k)
		}
	}
}

func (c *Cache) Set(k string, v interface{},d time.Duration){
	var e int64
	if d == DefaultExpiration{
		d = c.defaultExp
	}
	if d > 0{
		e = time.Now().Add(d).UnixNano()
	}
	c.mu.Lock()
	c.items[k] = Item{
		Object: v,
		Expr:   e}
	c.mu.Unlock()//这里试试看不用defer
}

// 设置数据项, 没有锁操作
func (c *Cache) set(k string, v interface{},d time.Duration){
	var e int64
	if d == DefaultExpiration{
		d = c.defaultExp
	}
	if d > 0{
		e = time.Now().Add(d).UnixNano()
	}
	c.items[k] = Item{
		Object: v,
		Expr:   e}
}

//获取数据
func (c * Cache) get(k string) (interface{}, bool){
	item, found := c.items[k]
	if !found{
		return nil, false
	}
	if item.Expired(){
		return nil, false
	}
	return item.Object, true
}

// 添加数据项，如果数据项已经存在，则返回错误
func (c *Cache) Add(k string, v interface{}, d time.Duration) error{
	c.mu.Lock()
	defer c.mu.Unlock()
	_ , found := c.get(k)
	if found{
		return fmt.Errorf("Item %s already exists", k)
	}
	c.set(k, v, d)
	return nil
}

// 获取数据项 带读锁
func (c *Cache) Get(k string) (interface{}, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    item, found := c.items[k]
    if !found {
        return nil, false
    }
    if item.Expired() {
        return nil, false
    }
    return item.Object, true
}

// 替换一个存在的数据项
func (c *Cache) Replace(k string, v interface{}, d time.Duration)error{
	c.mu.Lock()
	defer c.mu.Unlock()
	_ , found := c.get(k)
	if !found {
		return fmt.Errorf("Item %s doesn't exists", k)
	}
	c.set(k, v, d)
	return nil
}

func (c *Cache) Delete(k string){
	c.mu.Lock()
	c.delete(k)
	c.mu.Unlock()
}

func (c *Cache) Save(w io.Writer) (err error){
	enc := gob.NewEncoder(w)
	defer func() {
		if x := recover(); x != nil{
			err = fmt.Errorf("Error registering item types with Gob library")
		}
	}()
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, v := range c.items{
		gob.Register(v.Object)//告诉gob注册类型
	}
	err = enc.Encode(&c.items)
	return
}

func (c *Cache) SaveToFile(file string) error{
	f, err := os.Create(file)
	if err != nil{
		return err
	}
	if err = c.Save(f); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// 从 io.Reader 中读取数据项 读到items
func (c *Cache) Load(r io.Reader) error{
	dec := gob.NewDecoder(r)
	items := map[string]Item{}
	err := dec.Decode(&items)
	if err == nil {
		c.mu.Lock()
		defer c.mu.Unlock()
		for k, v := range items {
			ov, found := c.items[k]
			if found || !ov.Expired(){
				c.items[k] = v
			}
		}
	}
	return err
}

func (c *Cache) LoadFile(file string) error{
	f, err := os.Open(file)
	if err != nil{
		return err
	}
	if err = c.Load(f); err != nil{
		f.Close()
		return err
	}
	return f.Close()
}

func (c *Cache) Count() int {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return len(c.items)
}

// 清空缓存
func (c *Cache) Flush() {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.items = map[string]Item{}
}

// 停止过期缓存清理
func (c *Cache) StopGc() {
    c.stopGc <- true
}

// 创建一个缓存系统 
func NewCache(defaultExp, gcInterval time.Duration) *Cache {
    c := &Cache{
        defaultExp:   defaultExp,
        gcInterval:   gcInterval,
        items:        map[string]Item{},
        stopGc:       make(chan bool),
    }
    // 开始启动过期清理 goroutine
    go c.gcLoop()
    return c
}