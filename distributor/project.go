package distributor

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dearcode/crab/http/server"
	"github.com/dearcode/crab/orm"
	"github.com/zssky/log"

	"github.com/dearcode/sapper/distributor/config"
	"github.com/dearcode/sapper/util"
	"github.com/dearcode/sapper/util/etcd"
)

type project struct {
	ID     int64
	Source string
	Name   string
	Deploy []deploy `db_table:"one2more"`
	Ctime  string   `db_default:"now()"`
}

//GET 获取project的部署情况.
func (p *project) GET(w http.ResponseWriter, r *http.Request) {
	vars := struct {
		ID int64
	}{}

	if err := server.ParseURLVars(r, &vars); err != nil {
		server.SendResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	db, err := mdb.GetConnection()
	if err != nil {
		log.Errorf("connect db error:%v", err.Error())
		server.SendResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer db.Close()

	if err = orm.NewStmt(db, "project").Where("id=%v", vars.ID).Query(p); err != nil {
		log.Errorf("query project:%v error:%v", vars.ID, err.Error())
		server.SendResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	log.Debugf("project:%+v, id:%v", p, vars.ID)

	c, err := etcd.New(strings.Split(config.Distributor.ETCD.Hosts, ","))
	if err != nil {
		log.Errorf("etcd connect:%s error:%v", config.Distributor.ETCD.Hosts, err.Error())
		server.SendResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer c.Close()

	prefix := fmt.Sprintf("/api%s", p.Name)
	keys, err := c.List(prefix)
	if err != nil {
		log.Errorf("etcd list:%s error:%v", prefix, err.Error())
		server.SendResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	log.Debugf("keys:%v, prefix:%v", keys, prefix)

	server.SendResponseData(w, keys)
}

func (p *project) key() string {
	s := fmt.Sprintf("%x.%v", p.ID, time.Now().UnixNano())
	aesKey := []byte(config.Distributor.Server.SecretKey)
	aesKey = append(aesKey, []byte(strings.Repeat("\x00", 8-len(aesKey)%8))...)
	key, err := util.AesEncrypt([]byte(s), aesKey)
	if err != nil {
		log.Errorf("AesEncrypt key:%s, error:%s", s, config.Distributor.Server.SecretKey, err)
		return ""
	}

	return key
}
