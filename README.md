# gosession
基于Go实现的Session组件，代码参考了Beego的Session部分。目前实现了基于内存的Session和基于文件的Session。

通过实现SessionHandler和SessionProvider接口，可以扩展更多的储存方式。

使用方法：

```
  http.HandleFunc("/",helloWorld);
 	config := gosession.SessionConfig{
 		CookieName : "GO_SESSIONID",
 		EnableSetCookie : true,
 		Gclifetime : 10,
 		Maxlifetime : 10,
 		Secure : false,
 		CookieLifeTime : 3600,
 		ProviderConfig : "/mydata/session",
 		Domain : "",
 	}
 
 	manager,_ = gosession.NewSessionManager("file",config);
 
 
 
 	err := http.ListenAndServe(":8000",nil);
 
 	if(err != nil){
 		fmt.Println(err);
 	}
 	fmt.Println("应用已启动")
 	```
 helloWord实现：
 
 ```
     func helloWorld(w http.ResponseWriter, r *http.Request)  {
 
        r.ParseForm();
     
        session,_ := manager.SessionStart(w,r);
        session.Add("abc","dddddddddddddddddddd");
     
        value,_ := session.Get("abc");
     
        fmt.Fprintf(w,"hellow World!",value);
    }
 ```
