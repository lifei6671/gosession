package gosession

import (
	"sync"
	"time"
	"os"
	"path"
	"io/ioutil"
	"encoding/gob"
	"path/filepath"
	"errors"
)

var(
	fileProvider = &SessionFileProvider{}
)

type SessionFileStore struct {
	sessionId string
	rwMutex *sync.RWMutex
	values map[string]interface{}
	lastAccess time.Time
	savePath string
}

func (store *SessionFileStore) Add(key string,value interface{}) error {
	store.rwMutex.Lock();
	defer store.rwMutex.Unlock();

	store.values[key] = value;
	store.lastAccess = time.Now();

	err := sessionEncoder(store);

	if(err != nil){
		return err;
	}
	return nil;
}
func (store *SessionFileStore)Get(key string) (interface{},error)  {
	store.rwMutex.RLock();
	defer store.rwMutex.RUnlock();

	if value,ok := store.values[key];ok{
		return value,nil;
	}
	return nil,errors.New("值不存在");
}

func (store *SessionFileStore)Remove(key string) error  {
	store.rwMutex.Lock();
	defer store.rwMutex.Unlock();

	delete(store.values,key);

	err := sessionEncoder(store);

	if(err != nil){
		return err;
	}
	return nil;
}
func (store *SessionFileStore) Clear()  {
	store.rwMutex.Lock();
	defer store.rwMutex.Unlock();
	store.values = make(map[string]interface{});
	store.lastAccess = time.Now();

	sessionEncoder(store);
}

func (store *SessionFileStore) SessionId() string{
	return store.sessionId;
}
//将Session序列化到文件中
func sessionEncoder(store *SessionFileStore)  error {

	f,err := openFile(store.savePath);

	defer f.Close();
	if(err != nil){
		return err;
	}

	encoder := gob.NewEncoder(f);

	err = encoder.Encode(store.values)

	if(err != nil){
		return err;
	}
	return nil;
}

type SessionFileProvider struct {
	rwMutex  *sync.RWMutex
	maxlifetime int64
	savePath string
}

func (provider *SessionFileProvider)SessionInit(maxlifetime int64,savePath string) error{

	err := os.MkdirAll(savePath,os.ModePerm);
	if(err != nil){
		return err;
	}

	provider.maxlifetime = maxlifetime;
	provider.rwMutex = new(sync.RWMutex);
	provider.savePath = savePath;

	return nil;
}

func (provider *SessionFileProvider) Open(sessionId string) (SessionHandler,error)  {
	savePath := path.Join(provider.savePath,sessionId);

	f,err := openFile(savePath);
	defer f.Close();
	if(err != nil){

		return nil,err;
	}
	var kv map[string]interface{};

	content,err := ioutil.ReadFile(savePath);

	if(err != nil){
		return nil,err;
	}
	if len(content) == 0{
		kv = make(map[string]interface{});
	}else{
		decoder := gob.NewDecoder(f);

		err := decoder.Decode(&kv);

		if(err != nil){
			return nil,err;
		}
	}
	store := &SessionFileStore{
		sessionId:sessionId,
		rwMutex : new(sync.RWMutex),
		savePath : savePath,
		values:kv,
		lastAccess : time.Now(),
	};
	return store,nil;
}

func (provider *SessionFileProvider)Destroy(sessionId string) error  {
	savePath := path.Join(provider.savePath,sessionId);

	provider.rwMutex.Lock();
	defer provider.rwMutex.Unlock();

	err := os.Remove(savePath);
	return err;
}

func (provider *SessionFileProvider)GC()  {
	provider.rwMutex.Lock();
	defer provider.rwMutex.Unlock();

	filepath.Walk(provider.savePath,gcPath);
}

//判断文件过期时间并删除
func gcPath(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}
	if info.IsDir() {
		return nil
	}
	if (info.ModTime().Unix() + fileProvider.maxlifetime) < time.Now().Unix() {
		return os.Remove(path)
	}
	return nil
}

func openFile(savePath string) (*os.File,error) {
	_,err := os.Stat(savePath);

	var f *os.File;

	if(err == nil){
		f ,err = os.OpenFile(savePath,os.O_RDWR,0666);

		os.Chtimes(savePath,time.Now(),time.Now());

		if(err != nil){
			return f,err;
		}

	}else if( os.IsNotExist(err)){
		f,err = os.Create(savePath);

		if(err != nil){
			return f,err;
		}
	}else {
		return nil,err;
	}

	return f,nil;
}

func init()  {
	Register("file",fileProvider);
}