package repeater

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/coreos/etcd/mvcc/mvccpb"
	"github.com/dearcode/crab/http/client"
	"github.com/dearcode/crab/http/server"
	"github.com/juju/errors"
	"github.com/zssky/log"

	"github.com/dearcode/sapper/meta"
	"github.com/dearcode/sapper/repeater/config"
	"github.com/dearcode/sapper/util"
	"github.com/dearcode/sapper/util/etcd"
)

const (
	apigatePrefix = "/api"
)

type backendService struct {
	etcd *etcd.Client
	apps map[string][]meta.MicroAPP
	mu   sync.RWMutex
}

func newBackendService() (*backendService, error) {
	c, err := etcd.New(strings.Split(config.Repeater.ETCD.Hosts, ","))
	if err != nil {
		return nil, errors.Annotatef(err, config.Repeater.ETCD.Hosts)
	}

	return &backendService{etcd: c, apps: make(map[string][]meta.MicroAPP)}, nil
}

func (bs *backendService) start() {
	for {
		log.Debugf("begin watch key:%s", apigatePrefix)
		for _, e := range bs.etcd.WatchPrefix(apigatePrefix) {
			// /api/dbs/dbfree/handler/Fore/192.168.180.102/21638
			ss := strings.Split(string(e.Kv.Key), "/")
			if len(ss) < 4 {
				log.Errorf("invalid key:%s, event:%v", e.Kv.Key, e.Type)
				continue
			}

			//type只有DELETE和PUT.
			name := string(e.Kv.Key)
			name = name[4:strings.LastIndex(name, "/")]
			name = name[:strings.LastIndex(name, "/")]
			if e.Type == mvccpb.DELETE {
				port, _ := strconv.Atoi(ss[len(ss)-1])
				bs.unregister(name, ss[len(ss)-2], port)
				continue
			}

			app := meta.MicroAPP{}
			json.Unmarshal(e.Kv.Value, &app)
			bs.register(name, app)
		}
	}
}

func (bs *backendService) load() error {
	bss, err := bs.etcd.List(apigatePrefix)
	if err != nil {
		log.Errorf("list %s error:%v", apigatePrefix, err)
		return errors.Annotatef(err, apigatePrefix)
	}

	for k, v := range bss {
		// k = /api/dbs/dbfree/handler/Fore/192.168.180.102/21638
		ss := strings.Split(k, "/")
		if len(ss) < 4 {
			log.Errorf("invalid key:%s", k)
			continue
		}

		//type只有DELETE和PUT.
		name := k[4:strings.LastIndex(k, "/")]
		name = name[:strings.LastIndex(name, "/")]
		app := meta.MicroAPP{}
		json.Unmarshal([]byte(v), &app)
		bs.register(name, app)
	}

	return nil
}

//unregister 如果etcd中事务是删除，这里就去管理处删除.
func (bs *backendService) unregister(name, host string, port int) {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	apps, ok := bs.apps[name]
	if !ok {
		log.Debugf("cache app:%s not found", name)
		return
	}

	for i, app := range apps {
		if app.Host == host && app.Port == port {
			log.Infof("remove app:%s, addr:%v:%v", name, host, port)
			//只有他自己，直接删除了.
			if len(apps) == 1 {
				delete(bs.apps, name)
				return
			}
			ns := []meta.MicroAPP{}
			ns = append(ns, apps[:i]...)
			ns = append(ns, apps[i+1:]...)
			bs.apps[name] = ns
			return
		}
	}
}

type managerClient struct {
}

func (mc *managerClient) interfaceRegister(projectID int64, name, method, path, backend string) error {
	url := fmt.Sprintf("%sinterface/register/", config.Repeater.Manager.URL)
	req := struct {
		Name      string
		ProjectID int64
		Path      string
		Method    server.Method
		Backend   string
	}{
		Name:      name,
		ProjectID: projectID,
		Path:      path,
		Backend:   backend,
		Method:    server.NewMethod(method),
	}

	resp := struct {
		Status  int
		Data    int
		Message string
	}{}

	if err := client.New(config.Repeater.Server.Timeout).PostJSON(url, nil, req, &resp); err != nil {
		return errors.Annotatef(err, url)
	}

	if resp.Status != 0 {
		return errors.New(resp.Message)
	}

	log.Debugf("register success, id:%v", resp.Data)

	return nil
}

const (
	httpConnectTimeout = 60
)

type docField struct {
	Name     string
	Type     string
	Required bool
	Comment  string
}

type docMethod struct {
	Request map[string]docField
}

type docObject struct {
	URL     string
	Methods map[string]docMethod
}

func (bs *backendService) parseDocument(app meta.MicroAPP) error {
	url := fmt.Sprintf("http://%s:%d/document/", app.Host, app.Port)
	buf, err := client.New(httpConnectTimeout).Get(url, nil, nil)
	if err != nil {
		return errors.Trace(err)
	}

	var doc map[string]docObject

	if err = json.Unmarshal(buf, &doc); err != nil {
		log.Errorf("Unmarshal doc:%s error:%v", buf, err)
		return errors.Annotatef(err, "%s", buf)
	}

	log.Debugf("doc:%+v", doc)

	projectID, err := parseProjectID(app.ServiceKey)
	if err != nil {
		log.Errorf("parseProjectID:%s error:%v", app.ServiceKey, err)
		return errors.Annotatef(err, app.ServiceKey)
	}

	mc := managerClient{}

	for ok, ov := range doc {
		for mk := range ov.Methods {
			mc.interfaceRegister(projectID, ok+"_"+mk, mk, ov.URL, ov.URL)
		}
	}

	return nil
}

//register 到管理处添加接口, 肯定是多个repeater同时上报的，所以添加操作要指定版本信息.
func (bs *backendService) register(name string, app meta.MicroAPP) {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	apps, ok := bs.apps[name]
	if !ok {
		bs.apps[name] = []meta.MicroAPP{app}
		log.Debugf("new name:%s, app:%+v", name, app)
		bs.parseDocument(app)
		return
	}

	for _, o := range apps {
		if o.Host == app.Host && o.Port == app.Port {
			log.Errorf("invalid app:%v, apps:%#v", app, apps)
			return
		}
	}

	bs.apps[name] = append(apps, app)

	log.Debugf("new name:%s, add app:%+v", name, app)
	return
}

//getMicroAPPs 根据接口名获取后端应用列表.
func (bs *backendService) getMicroAPPs(name string) ([]meta.MicroAPP, error) {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	apps, ok := bs.apps[name]
	if !ok {
		return nil, errors.Annotatef(errNotFound, name)
	}

	return apps, nil
}

func (bs *backendService) stop() {
	bs.etcd.Close()
}

func parseProjectID(key string) (int64, error) {
	aesKey := []byte(config.Repeater.Server.SecretKey)
	aesKey = append(aesKey, []byte(strings.Repeat("\x00", 8-len(aesKey)%8))...)

	buf, err := util.AesDecrypt(key, aesKey)
	if err != nil {
		return 0, errors.Trace(err)
	}

	var id int64
	_, err = fmt.Sscanf(string(buf), "%x.", &id)
	if err != nil {
		return 0, errors.Trace(err)
	}

	return id, nil
}
