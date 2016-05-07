package gosession

import (
	"sync"
	"container/list"
	"time"
	"errors"
)

var memoryProvider = &SessionMemoryProvider{ lruList:list.New(), container : make(map[string]*list.Element,10), maxlifetime: 1800 ,rwNutex : sync.RWMutex{}}

type SessionMemoryStore struct {
	sessionId string
	values map[string]interface{}
	lastAccess time.Time
	rwMutex *sync.RWMutex
}
//添加一个值
func (store *SessionMemoryStore) Add(key string,value interface{}) error {
	store.rwMutex.Lock();
	defer store.rwMutex.Unlock();
	store.values[key] = value;
	store.lastAccess = time.Now();
	return nil;
}
func (store *SessionMemoryStore)Get(key string) (interface{},error)  {
	store.rwMutex.RLock();
	defer store.rwMutex.RUnlock();

	if value,ok := store.values[key];ok{
		return value,nil;
	}
	return nil,errors.New("值不存在");
}
func (store *SessionMemoryStore)Remove(key string) error{
	store.rwMutex.Lock();
	defer store.rwMutex.Unlock();
	delete(store.values,key);
	store.lastAccess = time.Now();

	return nil;
}
func (store *SessionMemoryStore) Clear()  {
	store.rwMutex.Lock();
	defer store.rwMutex.Unlock();
	store.values = make(map[string]interface{});
	store.lastAccess = time.Now();

}

func (store *SessionMemoryStore) SessionId() string{
	return store.sessionId;
}

type SessionMemoryProvider struct {
	container map[string]*list.Element
	rwNutex  sync.RWMutex
	maxlifetime int64
	lruList *list.List
}

//实现Session管道的初始化工作
func (provider *SessionMemoryProvider) SessionInit(maxlifetime int64,savePath string) error  {
	provider.container = make(map[string]*list.Element,10);
	provider.lruList = list.New();
	provider.maxlifetime = maxlifetime;
	provider.rwNutex = sync.RWMutex{}

	return nil;
}
//获取一个Session处理对象
func (provider *SessionMemoryProvider) Open(sessionId string) (SessionHandler,error)  {
	provider.rwNutex.RLock();
	if mem,ok := provider.container[sessionId];ok{
		provider.rwNutex.RUnlock()
		go func(){
			provider.rwNutex.Lock();
			if element,ok:=provider.container[sessionId];ok{
				provider.lruList.MoveToFront(element);
			}
			provider.rwNutex.Unlock();
		}();
		return mem.Value.(*SessionMemoryStore),nil;
	}
	provider.rwNutex.RUnlock();
	provider.rwNutex.Lock();

	handler := &SessionMemoryStore{ sessionId : sessionId, values: make(map[string]interface{}),lastAccess:time.Now(),rwMutex : &sync.RWMutex{}}

	element := provider.lruList.PushFront(handler);
	provider.container[sessionId] = element;

	provider.rwNutex.Unlock();

	return handler,nil;
}
func (provider *SessionMemoryProvider) Destroy(sessionId string) error{
	provider.rwNutex.Lock();
	defer provider.rwNutex.Unlock();

	if el,ok := provider.container[sessionId];ok{
		delete(provider.container,sessionId);
		provider.lruList.Remove(el);
		return nil;
	}
	return nil;
}

func (provider *SessionMemoryProvider) GC()  {
	provider.rwNutex.RLock();
	for {
		element := provider.lruList.Back();
		if(element ==nil){
			break;
		}

		if store := element.Value.(*SessionMemoryStore); (store.lastAccess.Unix()+provider.maxlifetime) < time.Now().Unix(){
			provider.rwNutex.RUnlock();
			provider.rwNutex.Lock();
			delete(provider.container,store.sessionId);
			provider.lruList.Remove(element);
			provider.rwNutex.Unlock();
			provider.rwNutex.RLock();
		}else{
			break;
		}
	}
	provider.rwNutex.RUnlock();
}

func init()  {
	Register("memory",memoryProvider);
}