package repeater

import (
	"github.com/dearcode/crab/cache"
	"github.com/dearcode/crab/orm"
	"github.com/juju/errors"

	"github.com/dearcode/sapper/repeater/config"
)

var (
	//Server 对外入口
	Server *repeater
	mdb    *orm.DB
	dc     *dbCache
)

//repeater 网关验证模块
type repeater struct {
}

//Init 返回验证模块
func Init() {
}

// ServerInit 初始化HTTP接口.
func ServerInit() error {
	if err := config.Load(); err != nil {
		return errors.Trace(err)
	}

	mdb = orm.NewDB(config.Repeater.DB.IP, config.Repeater.DB.Port, config.Repeater.DB.Name, config.Repeater.DB.User, config.Repeater.DB.Passwd, config.Repeater.DB.Charset, 10)

	dc = &dbCache{cache: cache.NewCache(int64(config.Repeater.Cache.Timeout))}
	if err := dc.conectDB(); err != nil {
		return errors.Trace(err)
	}

	Server = &repeater{}

	return nil
}